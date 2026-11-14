package quake

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/singularityos-lab/sinty-aegis/internal/alert"
	"github.com/singularityos-lab/sinty-aegis/internal/geo"
)

const USGS = "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/2.5_hour.geojson"
const EMSC = "https://www.seismicportal.eu/fdsnws/event/1/query?limit=40&format=json&minmag=2.5"

type Event struct {
	ID        string
	Source    string
	Time      time.Time
	Point     geo.Point
	Depth     float64
	Magnitude float64
	Place     string
	Tsunami   bool
	Link      string
}

type usgsDoc struct {
	Features []struct {
		ID         string `json:"id"`
		Properties struct {
			Mag     *float64 `json:"mag"`
			Place   string   `json:"place"`
			Time    int64    `json:"time"`
			URL     string   `json:"url"`
			Tsunami int      `json:"tsunami"`
			Type    string   `json:"type"`
		} `json:"properties"`
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

func ParseUSGS(data []byte) ([]Event, error) {
	var d usgsDoc
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	var out []Event
	for _, f := range d.Features {
		p := f.Properties
		if p.Mag == nil || len(f.Geometry.Coordinates) < 3 || p.Type != "earthquake" {
			continue
		}
		c := f.Geometry.Coordinates
		out = append(out, Event{ID: "usgs/" + f.ID, Source: "USGS", Time: time.UnixMilli(p.Time).UTC(), Point: geo.Point{Lat: c[1], Lon: c[0]}, Depth: c[2], Magnitude: *p.Mag, Place: p.Place, Tsunami: p.Tsunami == 1, Link: p.URL})
	}
	return out, nil
}

type emscDoc struct {
	Features []struct {
		Properties struct {
			Unid   string  `json:"unid"`
			Time   string  `json:"time"`
			Region string  `json:"flynn_region"`
			Lat    float64 `json:"lat"`
			Lon    float64 `json:"lon"`
			Depth  float64 `json:"depth"`
			Mag    float64 `json:"mag"`
			Type   string  `json:"evtype"`
		} `json:"properties"`
	} `json:"features"`
}

func ParseEMSC(data []byte) ([]Event, error) {
	var d emscDoc
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	var out []Event
	for _, f := range d.Features {
		p := f.Properties
		if p.Unid == "" || p.Type != "ke" {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, p.Time)
		if err != nil {
			continue
		}
		out = append(out, Event{ID: "emsc/" + p.Unid, Source: "EMSC", Time: t.UTC(), Point: geo.Point{Lat: p.Lat, Lon: p.Lon}, Depth: math.Abs(p.Depth), Magnitude: p.Mag, Place: titleCase(p.Region), Link: "https://www.seismicportal.eu/eventdetails.html?unid=" + p.Unid})
	}
	return out, nil
}

func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func Radius(magnitude float64) float64 {
	if magnitude < 2 {
		return 0
	}
	return math.Round(math.Pow(10, 0.5*magnitude-0.2))
}

func Felt(e Event, here geo.Point, minMagnitude float64) (float64, bool) {
	d := geo.Distance(e.Point, here)
	return d, e.Magnitude >= minMagnitude && d <= Radius(e.Magnitude)
}

func Same(a, b Event) bool {
	dt := a.Time.Sub(b.Time)
	if dt < 0 {
		dt = -dt
	}
	return dt <= 90*time.Second && geo.Distance(a.Point, b.Point) <= 100
}

func Severity(magnitude, km float64) alert.Severity {
	switch {
	case magnitude >= 6.5 || magnitude >= 5.5 && km <= 60:
		return alert.Extreme
	case magnitude >= 5 || magnitude >= 4.5 && km <= 40:
		return alert.Severe
	case magnitude >= 4:
		return alert.Moderate
	}
	return alert.Minor
}

func (e Event) Alert(km float64, raisedID string) alert.Alert {
	return alert.Alert{
		ID:          raisedID,
		Kind:        alert.Earthquake,
		Severity:    Severity(e.Magnitude, km),
		Headline:    fmt.Sprintf("M %.1f, %s", e.Magnitude, e.Place),
		Description: fmt.Sprintf("%.0f km away, %.0f km deep, %s UTC", km, e.Depth, e.Time.Format("15:04")),
		Area:        e.Place,
		Source:      e.Source,
		Link:        e.Link,
		Sent:        e.Time,
		Expires:     e.Time.Add(2 * time.Hour),
	}
}
