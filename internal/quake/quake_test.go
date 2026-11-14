package quake

import (
	"os"
	"testing"
	"time"

	"github.com/singularityos-lab/sinty-aegis/internal/alert"
	"github.com/singularityos-lab/sinty-aegis/internal/geo"
)

func TestParseUSGS(t *testing.T) {
	data, err := os.ReadFile("testdata/usgs.json")
	if err != nil {
		t.Fatal(err)
	}
	events, err := ParseUSGS(data)
	if err != nil || len(events) == 0 {
		t.Fatalf("events = %d err = %v", len(events), err)
	}
	e := events[0]
	if e.Magnitude != 5.5 || e.Place != "northern Mid-Atlantic Ridge" || e.Point.Lat != 30.122 || e.Point.Lon != -42.6346 || e.Time.IsZero() {
		t.Fatalf("first event = %+v", e)
	}
}

func TestParseEMSC(t *testing.T) {
	data, err := os.ReadFile("testdata/emsc.json")
	if err != nil {
		t.Fatal(err)
	}
	events, err := ParseEMSC(data)
	if err != nil || len(events) == 0 {
		t.Fatalf("events = %d err = %v", len(events), err)
	}
	e := events[0]
	if e.ID != "emsc/20261114_0000118" || e.Place != "South Africa" || e.Magnitude != 2.5 || e.Point.Lat != -28.321 {
		t.Fatalf("first event = %+v", e)
	}
}

func TestFeltAndSeverity(t *testing.T) {
	rome := geo.Point{Lat: 41.9, Lon: 12.5}
	near := Event{Magnitude: 5.8, Point: geo.Point{Lat: 42.4, Lon: 13.3}, Time: time.Unix(1000, 0)}
	km, felt := Felt(near, rome, 3.5)
	if !felt || km < 60 || km > 110 {
		t.Fatalf("near quake km = %.0f felt = %v", km, felt)
	}
	if _, felt := Felt(Event{Magnitude: 3.0, Point: near.Point}, rome, 3.5); felt {
		t.Fatal("quake under the threshold reported")
	}
	if _, felt := Felt(Event{Magnitude: 4.0, Point: geo.Point{Lat: 35.6, Lon: 139.7}}, rome, 3.5); felt {
		t.Fatal("small quake on the other side of the world reported")
	}
	if Severity(6.8, 300) != alert.Extreme || Severity(5.1, 200) != alert.Severe || Severity(4.2, 80) != alert.Moderate || Severity(3.6, 10) != alert.Minor {
		t.Fatal("severity scale wrong")
	}
	other := Event{Magnitude: 5.7, Point: geo.Point{Lat: 42.45, Lon: 13.35}, Time: time.Unix(1030, 0)}
	if !Same(near, other) || Same(near, Event{Point: near.Point, Time: time.Unix(5000, 0)}) {
		t.Fatal("duplicate detection wrong")
	}
}
