package geo

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestDistance(t *testing.T) {
	rome := Point{41.9028, 12.4964}
	milan := Point{45.4642, 9.19}
	if d := Distance(rome, milan); math.Abs(d-477) > 3 {
		t.Fatalf("Rome to Milan = %.1f km", d)
	}
}

func TestPolygon(t *testing.T) {
	poly := ParsePolygon("45.0,7.0 45.0,10.0 42.0,10.0 42.0,7.0 45.0,7.0")
	if poly == nil {
		t.Fatal("polygon not parsed")
	}
	if !InPolygon(Point{43.5, 8.5}, poly) {
		t.Fatal("inside point reported outside")
	}
	if InPolygon(Point{46, 8.5}, poly) {
		t.Fatal("outside point reported inside")
	}
	if ParsePolygon("45.0,7.0 bad") != nil {
		t.Fatal("bad polygon accepted")
	}
}

func TestCircle(t *testing.T) {
	c, ok := ParseCircle("41.9,12.5 50")
	if !ok {
		t.Fatal("circle not parsed")
	}
	if !c.Contains(Point{42.1, 12.6}) || c.Contains(Point{45.4, 9.2}) {
		t.Fatal("circle membership wrong")
	}
}

func TestZoneFor(t *testing.T) {
	tab := filepath.Join(t.TempDir(), "zone1970.tab")
	os.WriteFile(tab, []byte("# comment\nIT,SM,VA\t+4154+01229\tEurope/Rome\nUS\t+404251-0740023\tAmerica/New_York\tEastern\n"), 0o644)
	z, ok := ZoneFor("Europe/Rome", tab)
	if !ok || z.Country != "it" || math.Abs(z.Point.Lat-41.9) > 0.01 || math.Abs(z.Point.Lon-12.4833) > 0.01 {
		t.Fatalf("Rome zone = %+v %v", z, ok)
	}
	z, ok = ZoneFor("America/New_York", tab)
	if !ok || z.Country != "us" || math.Abs(z.Point.Lon+74.0064) > 0.01 {
		t.Fatalf("New York zone = %+v %v", z, ok)
	}
	if _, ok := ZoneFor("Mars/Olympus", tab); ok {
		t.Fatal("unknown zone resolved")
	}
}
