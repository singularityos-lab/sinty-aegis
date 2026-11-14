package cap

import (
	"os"
	"testing"

	"github.com/singularityos-lab/sinty-aegis/internal/alert"
	"github.com/singularityos-lab/sinty-aegis/internal/geo"
)

func TestMeteoalarmMessage(t *testing.T) {
	data, err := os.ReadFile("testdata/meteoalarm-italy.xml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Actionable() || m.Cancels() {
		t.Fatalf("status %q type %q scope %q not actionable", m.Status, m.MsgType, m.Scope)
	}
	it, ok := m.Pick([]string{"it"})
	if !ok || it.Language != "it-IT" {
		t.Fatalf("picked %q for it", it.Language)
	}
	en, _ := m.Pick([]string{"fr", "en"})
	if en.Language != "en-GB" {
		t.Fatalf("picked %q for fr,en", en.Language)
	}
	if covered, geometric := en.Covers(geo.Point{Lat: 39.2, Lon: 9.1}); covered || geometric {
		t.Fatal("named area reported as geometric")
	}
	a := m.Alert(en, "fallback")
	if a.Kind != alert.Storm || a.Severity != alert.Severe || a.Area != "Sardegna" {
		t.Fatalf("alert = %+v", a)
	}
	if a.Source != "Italian Air Force National Meteorological Service" || a.Expires.IsZero() || a.Sent.IsZero() || a.Instruction == "" {
		t.Fatalf("alert fields = %+v", a)
	}
}

func TestFeedLinks(t *testing.T) {
	data, err := os.ReadFile("testdata/meteoalarm-feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	links, err := FeedLinks(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 2 {
		t.Fatalf("links = %v", links)
	}
	for _, l := range links {
		if len(l) < 60 || l[:47] != "https://feeds.meteoalarm.org/api/v1/warnings/fe" {
			t.Fatalf("link = %q", l)
		}
	}
}

func TestRSSLinks(t *testing.T) {
	links, err := FeedLinks([]byte(`<rss><channel><item><title>a</title><link>https://example.org/a.xml</link></item><item><link> https://example.org/b.xml </link></item></channel></rss>`))
	if err != nil || len(links) != 2 || links[1] != "https://example.org/b.xml" {
		t.Fatalf("links = %v err = %v", links, err)
	}
}

func TestGeometryAndFilters(t *testing.T) {
	m, err := Parse([]byte(`<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>x1</identifier><sender>s</sender><sent>2026-11-14T10:00:00+00:00</sent><status>Test</status><msgType>Alert</msgType><scope>Public</scope><info><language>en</language><category>Geo</category><event>Tsunami Warning</event><severity>Extreme</severity><headline>Tsunami</headline><area><areaDesc>Coast</areaDesc><polygon>45.0,7.0 45.0,10.0 42.0,10.0 42.0,7.0 45.0,7.0</polygon><circle>10.0,10.0 5</circle></area></info></alert>`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Actionable() {
		t.Fatal("test message reported actionable")
	}
	i := m.Infos[0]
	if covered, geometric := i.Covers(geo.Point{Lat: 43.5, Lon: 8.5}); !covered || !geometric {
		t.Fatal("point inside polygon not covered")
	}
	if covered, _ := i.Covers(geo.Point{Lat: 50, Lon: 8.5}); covered {
		t.Fatal("point outside covered")
	}
	if covered, _ := i.Covers(geo.Point{Lat: 10.01, Lon: 10.01}); !covered {
		t.Fatal("point inside circle not covered")
	}
	if Kind(i) != alert.Tsunami || Severity(i.Severity) != alert.Extreme {
		t.Fatalf("kind %v severity %v", Kind(i), Severity(i.Severity))
	}
}
