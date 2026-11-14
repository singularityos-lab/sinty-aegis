package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/singularityos-lab/sinty-aegis/internal/alert"
	"github.com/singularityos-lab/sinty-aegis/internal/config"
	"github.com/singularityos-lab/sinty-aegis/internal/quake"
	"github.com/singularityos-lab/sinty-aegis/internal/registry"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func engine(t *testing.T, cfg config.Config, pages map[string][]byte) *Engine {
	t.Helper()
	dir := t.TempDir()
	e := NewEngine(filepath.Join(dir, "aegis.json"), nil)
	e.Cache = filepath.Join(dir, "cache")
	e.cfg = cfg
	e.Fetch = func(ctx context.Context, url string) ([]byte, error) {
		if data, ok := pages[url]; ok {
			return data, nil
		}
		return nil, errors.New("offline: " + url)
	}
	return e
}

func italy(t *testing.T) map[string][]byte {
	feed := read(t, "../cap/testdata/meteoalarm-feed.xml")
	links := strings.Split(string(feed), `type="application/cap+xml" href="`)
	pages := map[string][]byte{
		registry.URL: read(t, "../registry/testdata/sources.json"),
		"https://feeds.meteoalarm.org/feeds/meteoalarm-legacy-atom-italy": feed,
	}
	for _, part := range links[1:] {
		url, _, _ := strings.Cut(part, `"`)
		pages[url] = read(t, "../cap/testdata/meteoalarm-italy.xml")
	}
	return pages
}

func future(data []byte) []byte {
	s := string(data)
	year := time.Now().AddDate(1, 0, 0).Format("2006")
	return []byte(strings.ReplaceAll(s, "<expires>2026-", "<expires>"+year+"-"))
}

func TestNamedAreaNeedsThePlace(t *testing.T) {
	pages := italy(t)
	for url, data := range pages {
		if strings.Contains(url, "/warnings/") {
			pages[url] = future(data)
		}
	}
	cfg := config.Default()
	cfg.Country, cfg.Lat, cfg.Lon, cfg.Languages = "it", 39.22, 9.12, []string{"it"}
	cfg.Places = []string{"Lombardia"}
	e := engine(t, cfg, pages)
	e.PollAlerts(context.Background())
	if n := len(e.Store.Active()); n != 0 {
		t.Fatalf("alert for Sardegna raised in Lombardia: %d", n)
	}
	areas := e.Areas()
	if len(areas) != 1 || areas[0] != "Sardegna" {
		t.Fatalf("areas = %v", areas)
	}
	cfg.Places = []string{"sardegna"}
	e = engine(t, cfg, pages)
	e.PollAlerts(context.Background())
	active := e.Store.Active()
	if len(active) != 1 {
		t.Fatalf("active = %+v", active)
	}
	a := active[0]
	if a.Kind != alert.Storm || a.Severity != alert.Severe || a.Language != "it-IT" || a.Area != "Sardegna" {
		t.Fatalf("alert = %+v", a)
	}
	e.PollAlerts(context.Background())
	if len(e.Store.Active()) != 1 {
		t.Fatal("second poll duplicated the alert")
	}
}

func TestExpiredAndDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Country, cfg.Lat, cfg.Lon, cfg.Places = "it", 39.22, 9.12, []string{"Sardegna"}
	pages := italy(t)
	for url, data := range pages {
		if strings.Contains(url, "/warnings/") {
			pages[url] = []byte(strings.ReplaceAll(string(data), "<expires>2026-", "<expires>2020-"))
		}
	}
	e := engine(t, cfg, pages)
	e.PollAlerts(context.Background())
	if len(e.Store.Active()) != 0 {
		t.Fatal("expired warning raised")
	}
	cfg.Enabled = false
	e = engine(t, cfg, italy(t))
	e.PollAlerts(context.Background())
	if len(e.Store.Active()) != 0 || len(e.Areas()) != 0 {
		t.Fatal("disabled engine fetched alerts")
	}
}

func TestQuakeNearby(t *testing.T) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.0Z")
	emsc := `{"features":[{"properties":{"unid":"near1","time":"` + now + `","flynn_region":"CENTRAL ITALY","lat":42.4,"lon":13.3,"depth":9,"mag":5.6,"evtype":"ke"}},{"properties":{"unid":"far1","time":"` + now + `","flynn_region":"FIJI REGION","lat":-17.8,"lon":178.0,"depth":500,"mag":5.9,"evtype":"ke"}}]}`
	cfg := config.Default()
	cfg.Country, cfg.Lat, cfg.Lon = "it", 41.9, 12.5
	e := engine(t, cfg, map[string][]byte{quake.EMSC: []byte(emsc)})
	e.PollQuakes(context.Background())
	active := e.Store.Active()
	if len(active) != 1 || active[0].Kind != alert.Earthquake || active[0].Severity != alert.Severe || !strings.HasPrefix(active[0].Headline, "M 5.6, Central Italy") {
		t.Fatalf("active = %+v", active)
	}
	if err := e.Store.Dismiss(active[0].ID); err != alert.ErrHeld {
		t.Fatalf("early dismiss = %v", err)
	}
}

func TestTestAlert(t *testing.T) {
	e := engine(t, config.Default(), nil)
	a := e.Test(alert.Tsunami)
	if !a.Test || a.Kind != alert.Tsunami || a.Severity != alert.Extreme || a.Instruction == "" {
		t.Fatalf("test alert = %+v", a)
	}
	if b := e.Test("nonsense"); b.Kind != alert.National {
		t.Fatalf("fallback kind = %v", b.Kind)
	}
	if len(e.Store.Active()) != 2 {
		t.Fatal("test alerts not active")
	}
}
