package cap

import (
	"encoding/xml"
	"strings"
	"time"

	"github.com/singularityos-lab/sinty-aegis/internal/alert"
	"github.com/singularityos-lab/sinty-aegis/internal/geo"
)

type Area struct {
	Desc     string   `xml:"areaDesc"`
	Polygons []string `xml:"polygon"`
	Circles  []string `xml:"circle"`
}

type Info struct {
	Language    string   `xml:"language"`
	Categories  []string `xml:"category"`
	Event       string   `xml:"event"`
	Urgency     string   `xml:"urgency"`
	Severity    string   `xml:"severity"`
	Certainty   string   `xml:"certainty"`
	Expires     string   `xml:"expires"`
	SenderName  string   `xml:"senderName"`
	Headline    string   `xml:"headline"`
	Description string   `xml:"description"`
	Instruction string   `xml:"instruction"`
	Web         string   `xml:"web"`
	Areas       []Area   `xml:"area"`
}

type Message struct {
	XMLName    xml.Name `xml:"alert"`
	Identifier string   `xml:"identifier"`
	Sender     string   `xml:"sender"`
	Sent       string   `xml:"sent"`
	Status     string   `xml:"status"`
	MsgType    string   `xml:"msgType"`
	Scope      string   `xml:"scope"`
	Infos      []Info   `xml:"info"`
}

func Parse(data []byte) (Message, error) {
	var m Message
	err := xml.Unmarshal(data, &m)
	return m, err
}

func (m Message) Actionable() bool {
	return strings.EqualFold(m.Status, "Actual") && strings.EqualFold(m.Scope, "Public") && !strings.EqualFold(m.MsgType, "Cancel") && !strings.EqualFold(m.MsgType, "Ack") && !strings.EqualFold(m.MsgType, "Error")
}

func (m Message) Cancels() bool {
	return strings.EqualFold(m.MsgType, "Cancel")
}

func (a Area) Geometric() bool {
	return len(a.Polygons)+len(a.Circles) > 0
}

func (a Area) Contains(p geo.Point) bool {
	for _, s := range a.Polygons {
		if poly := geo.ParsePolygon(s); poly != nil && geo.InPolygon(p, poly) {
			return true
		}
	}
	for _, s := range a.Circles {
		if c, ok := geo.ParseCircle(s); ok && c.Contains(p) {
			return true
		}
	}
	return false
}

func (i Info) Covers(p geo.Point) (covered bool, geometric bool) {
	for _, a := range i.Areas {
		if a.Geometric() {
			geometric = true
			if a.Contains(p) {
				return true, true
			}
		}
	}
	return false, geometric
}

func (m Message) Pick(languages []string) (Info, bool) {
	if len(m.Infos) == 0 {
		return Info{}, false
	}
	for _, want := range languages {
		want = strings.ToLower(want)
		for _, i := range m.Infos {
			have := strings.ToLower(i.Language)
			if have == want || strings.HasPrefix(have, want+"-") || strings.HasPrefix(want, have+"-") && have != "" {
				return i, true
			}
		}
	}
	return m.Infos[0], true
}

func Severity(s string) alert.Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "extreme":
		return alert.Extreme
	case "severe":
		return alert.Severe
	case "moderate":
		return alert.Moderate
	}
	return alert.Minor
}

var words = []struct {
	kind alert.Kind
	any  []string
}{
	{alert.Tsunami, []string{"tsunami", "maremoto"}},
	{alert.Earthquake, []string{"earthquake", "seism", "terremoto", "sismo", "tremblement"}},
	{alert.Volcano, []string{"volcan", "vulcan", "ashfall"}},
	{alert.Landslide, []string{"landslide", "avalanche", "frana", "valanga", "mudslide", "debris flow"}},
	{alert.Flood, []string{"flood", "alluvion", "inondation", "inundaci", "hochwasser", "idrogeologic", "idraulic", "storm surge", "high water"}},
	{alert.Fire, []string{"fire", "incend", "feu", "waldbrand"}},
	{alert.Snow, []string{"snow", "neve", "neige", "nieve", "ice", "ghiaccio", "blizzard", "schnee", "freezing rain"}},
	{alert.Heat, []string{"heat", "caldo", "chaleur", "calor", "hitze", "high temperature"}},
	{alert.Cold, []string{"cold", "freddo", "froid", "frost", "gelo", "low temperature"}},
	{alert.Wind, []string{"wind", "vento", "vent ", "viento", "gale", "tornado", "cyclone", "hurricane", "typhoon"}},
	{alert.Storm, []string{"storm", "thunder", "temporal", "orage", "tormenta", "gewitter", "rain", "pioggia", "pluie", "lluvia", "hail", "grandine"}},
	{alert.Health, []string{"health", "epidemi", "pandemi", "air quality", "contamin"}},
	{alert.Security, []string{"terror", "attack", "shooter", "civil danger", "evacuat", "missile", "nuclear", "radiolog", "chemical"}},
}

func Kind(i Info) alert.Kind {
	text := strings.ToLower(i.Event + " " + i.Headline)
	for _, w := range words {
		for _, s := range w.any {
			if strings.Contains(text, s) {
				return w.kind
			}
		}
	}
	for _, c := range i.Categories {
		switch strings.ToLower(strings.TrimSpace(c)) {
		case "fire":
			return alert.Fire
		case "health":
			return alert.Health
		case "security", "cbrne":
			return alert.Security
		case "met":
			return alert.Storm
		}
	}
	return alert.Other
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}
	}
	return t
}

func (m Message) Alert(i Info, source string) alert.Alert {
	areas := make([]string, 0, len(i.Areas))
	for _, a := range i.Areas {
		if d := strings.TrimSpace(a.Desc); d != "" {
			areas = append(areas, d)
		}
	}
	headline := strings.TrimSpace(i.Headline)
	if headline == "" {
		headline = strings.TrimSpace(i.Event)
	}
	if name := strings.TrimSpace(i.SenderName); name != "" {
		source = name
	}
	return alert.Alert{
		ID:          m.Sender + "/" + m.Identifier,
		Kind:        Kind(i),
		Severity:    Severity(i.Severity),
		Headline:    headline,
		Description: strings.TrimSpace(i.Description),
		Instruction: strings.TrimSpace(i.Instruction),
		Area:        strings.Join(areas, ", "),
		Source:      source,
		Link:        strings.TrimSpace(i.Web),
		Language:    i.Language,
		Sent:        parseTime(m.Sent),
		Expires:     parseTime(i.Expires),
	}
}

type feedEntry struct {
	Link  []feedLink `xml:"link"`
	GUID  string     `xml:"guid"`
	ID    string     `xml:"id"`
	Title string     `xml:"title"`
}

type feedLink struct {
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}

type feed struct {
	Items   []feedEntry `xml:"channel>item"`
	Entries []feedEntry `xml:"entry"`
}

func FeedLinks(data []byte) ([]string, error) {
	var f feed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	for _, e := range append(f.Items, f.Entries...) {
		best := ""
		for _, l := range e.Link {
			u := strings.TrimSpace(l.Href)
			if u == "" {
				u = strings.TrimSpace(l.Text)
			}
			if u == "" {
				continue
			}
			if best == "" || strings.Contains(l.Type, "cap") {
				best = u
			}
		}
		if best != "" && !seen[best] {
			seen[best] = true
			out = append(out, best)
		}
	}
	return out, nil
}
