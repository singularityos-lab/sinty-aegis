package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/singularityos-lab/sinty-aegis/internal/alert"
	"github.com/singularityos-lab/sinty-aegis/internal/cap"
	"github.com/singularityos-lab/sinty-aegis/internal/config"
	"github.com/singularityos-lab/sinty-aegis/internal/geo"
	"github.com/singularityos-lab/sinty-aegis/internal/quake"
	"github.com/singularityos-lab/sinty-aegis/internal/registry"
)

const ZoneTab = "/usr/share/zoneinfo/zone1970.tab"
const maxBody = 8 << 20
const perFeed = 80

type Fetcher func(ctx context.Context, url string) ([]byte, error)

type Engine struct {
	Store *alert.Store
	Fetch Fetcher
	Cache string
	Zone  string

	mu      sync.Mutex
	cfg     config.Config
	path    string
	sources []registry.Source
	loaded  time.Time
	read    map[string]bool
	quakes  []quake.Event
	areas   map[string]bool
}

func HTTP(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sinty-aegis")
	req.Header.Set("Accept", "application/cap+xml, application/atom+xml, application/rss+xml, application/json, */*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBody))
}

func NewEngine(path string, changed func()) *Engine {
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	return &Engine{
		Store: alert.NewStore(nil, changed),
		Fetch: HTTP,
		Cache: filepath.Join(cache, "singularity", "aegis"),
		Zone:  ZoneTab,
		cfg:   config.Load(path),
		path:  path,
		read:  map[string]bool{},
		areas: map[string]bool{},
	}
}

func (e *Engine) Config() config.Config {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg
}

func (e *Engine) SetConfig(c config.Config) error {
	c = c.Clean()
	e.mu.Lock()
	moved := c.Country != e.cfg.Country
	e.cfg = c
	if moved {
		e.read = map[string]bool{}
		e.areas = map[string]bool{}
	}
	path := e.path
	e.mu.Unlock()
	return c.Save(path)
}

func (e *Engine) Areas() []string {
	e.mu.Lock()
	out := make([]string, 0, len(e.areas))
	for a := range e.areas {
		out = append(out, a)
	}
	e.mu.Unlock()
	sort.Strings(out)
	return out
}

func (e *Engine) Sources() []registry.Source {
	e.mu.Lock()
	defer e.mu.Unlock()
	country, _, _ := e.cfg.Where(e.Zone)
	return registry.For(e.sources, country, e.languages())
}

func (e *Engine) languages() []string {
	if len(e.cfg.Languages) > 0 {
		return e.cfg.Languages
	}
	var out []string
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := os.Getenv(key)
		if lang, _, _ := strings.Cut(v, "_"); len(lang) == 2 {
			out = append(out, strings.ToLower(lang))
			break
		}
	}
	return append(out, "en")
}

func (e *Engine) loadRegistry(ctx context.Context) {
	e.mu.Lock()
	fresh := len(e.sources) > 0 && time.Since(e.loaded) < 24*time.Hour
	e.mu.Unlock()
	if fresh {
		return
	}
	file := filepath.Join(e.Cache, "sources.json")
	data, err := e.Fetch(ctx, registry.URL)
	if err == nil {
		os.MkdirAll(e.Cache, 0o700)
		os.WriteFile(file, data, 0o600)
	} else {
		log.Printf("registry: %v", err)
		data, err = os.ReadFile(file)
		if err != nil {
			return
		}
	}
	sources, err := registry.Parse(data)
	if err != nil {
		log.Printf("registry: %v", err)
		return
	}
	e.mu.Lock()
	e.sources, e.loaded = sources, time.Now()
	e.mu.Unlock()
}

func named(areas []cap.Area, places []string) bool {
	for _, a := range areas {
		desc := strings.ToLower(a.Desc)
		for _, p := range places {
			if p = strings.ToLower(strings.TrimSpace(p)); p != "" && strings.Contains(desc, p) {
				return true
			}
		}
	}
	return false
}

func (e *Engine) consider(m cap.Message, source string, cfg config.Config, here geo.Point, languages []string) {
	if !m.Actionable() {
		return
	}
	info, ok := m.Pick(languages)
	if !ok {
		return
	}
	e.mu.Lock()
	for _, a := range info.Areas {
		if d := strings.TrimSpace(a.Desc); d != "" && len(e.areas) < 2000 {
			e.areas[d] = true
		}
	}
	e.mu.Unlock()
	a := m.Alert(info, source)
	if int(a.Severity) < cfg.MinSeverity || cfg.Mutes(a.Kind) {
		return
	}
	if !a.Expires.IsZero() && time.Now().After(a.Expires) {
		return
	}
	covered, geometric := info.Covers(here)
	switch {
	case geometric && !covered:
		return
	case !geometric && len(cfg.Places) > 0 && !named(info.Areas, cfg.Places):
		return
	case !geometric && len(cfg.Places) == 0 && a.Severity < alert.Severe:
		return
	}
	e.Store.Raise(a)
}

func (e *Engine) PollAlerts(ctx context.Context) {
	cfg := e.Config()
	if !cfg.Enabled {
		return
	}
	e.loadRegistry(ctx)
	_, here, ok := cfg.Where(e.Zone)
	if !ok {
		return
	}
	e.mu.Lock()
	languages := e.languages()
	e.mu.Unlock()
	for _, src := range e.Sources() {
		data, err := e.Fetch(ctx, src.Feed)
		if err != nil {
			log.Printf("%s: %v", src.ID, err)
			continue
		}
		links, err := cap.FeedLinks(data)
		if err != nil {
			log.Printf("%s: %v", src.ID, err)
			continue
		}
		if len(links) > perFeed {
			links = links[:perFeed]
		}
		for _, link := range links {
			e.mu.Lock()
			done := e.read[link]
			e.mu.Unlock()
			if done || !strings.HasPrefix(link, "https://") {
				continue
			}
			body, err := e.Fetch(ctx, link)
			if err != nil {
				continue
			}
			e.mu.Lock()
			e.read[link] = true
			e.mu.Unlock()
			m, err := cap.Parse(body)
			if err != nil {
				continue
			}
			e.consider(m, src.Name, cfg, here, languages)
		}
	}
	e.Store.Expire()
}

func (e *Engine) PollQuakes(ctx context.Context) {
	cfg := e.Config()
	if !cfg.Enabled || cfg.Mutes(alert.Earthquake) {
		return
	}
	_, here, ok := cfg.Where(e.Zone)
	if !ok {
		return
	}
	var events []quake.Event
	if data, err := e.Fetch(ctx, quake.EMSC); err == nil {
		list, _ := quake.ParseEMSC(data)
		events = append(events, list...)
	} else {
		log.Printf("emsc: %v", err)
	}
	if data, err := e.Fetch(ctx, quake.USGS); err == nil {
		list, _ := quake.ParseUSGS(data)
		events = append(events, list...)
	} else {
		log.Printf("usgs: %v", err)
	}
	for _, ev := range events {
		if time.Since(ev.Time) > 2*time.Hour {
			continue
		}
		km, felt := quake.Felt(ev, here, cfg.MinMagnitude)
		if !felt {
			continue
		}
		id := ev.ID
		e.mu.Lock()
		for _, old := range e.quakes {
			if quake.Same(old, ev) {
				id = old.ID
				break
			}
		}
		if id == ev.ID {
			e.quakes = append(e.quakes, ev)
			if len(e.quakes) > 200 {
				e.quakes = e.quakes[len(e.quakes)-200:]
			}
		}
		e.mu.Unlock()
		a := ev.Alert(km, id)
		if int(a.Severity) >= cfg.MinSeverity {
			e.Store.Raise(a)
		}
	}
	e.Store.Expire()
}

var samples = map[alert.Kind][3]string{
	alert.Earthquake: {"M 5.8, 12 km from you", "Strong shaking is possible.", "Drop, cover and hold on. Stay away from windows."},
	alert.Tsunami:    {"Tsunami warning for your coast", "A tsunami is possible after a strong earthquake at sea.", "Move to high ground now. Do not wait for the first wave."},
	alert.Flood:      {"Flood warning in your area", "Rivers are rising quickly.", "Leave basements and low ground. Do not drive through water."},
	alert.Fire:       {"Wildfire near you", "A fire is spreading with the wind.", "Be ready to leave. Close windows and follow the authorities."},
	alert.Storm:      {"Severe thunderstorm warning", "Heavy rain, hail and lightning are expected.", "Stay indoors and away from trees."},
	alert.Wind:       {"Violent wind warning", "Gusts strong enough to bring down trees and power lines.", "Stay inside and away from windows."},
	alert.Snow:       {"Blizzard warning", "Heavy snow and ice will make roads impassable.", "Do not travel. Keep warm and keep your phone charged."},
	alert.Heat:       {"Extreme heat warning", "Dangerous temperatures are expected in the afternoon.", "Stay in the shade, drink water and check on older neighbours."},
	alert.Cold:       {"Extreme cold warning", "Temperatures will fall far below freezing tonight.", "Stay indoors and protect water pipes."},
	alert.Volcano:    {"Volcanic eruption warning", "Ash fall is expected downwind of the volcano.", "Stay indoors, close windows and wear a mask outside."},
	alert.Landslide:  {"Landslide warning", "The ground on steep slopes is unstable after the rain.", "Leave houses below steep slopes and stay off mountain roads."},
	alert.Health:     {"Public health alert", "Tap water in your area may be unsafe to drink.", "Boil water before drinking it until further notice."},
	alert.Security:   {"Public safety alert", "The police ask everyone in the area to stay inside.", "Lock doors, stay away from windows and wait for the all clear."},
	alert.National:   {"National alert", "A message from your national authority.", "Follow the instructions of the authorities."},
	alert.Other:      {"Emergency alert", "An authority issued an alert for your area.", "Follow the instructions of the authorities."},
}

func (e *Engine) Test(kind alert.Kind, danger bool) alert.Alert {
	s, ok := samples[kind]
	if !ok {
		kind, s = alert.National, samples[alert.National]
	}
	severity := alert.Extreme
	if !danger {
		severity = alert.Moderate
	}
	a := alert.Alert{ID: fmt.Sprintf("test/%s/%d", kind, time.Now().UnixNano()), Kind: kind, Severity: severity, Headline: s[0], Description: s[1] + " This is a test of Sinty Aegis.", Instruction: s[2], Area: "Test", Source: "Sinty Aegis", Sent: time.Now(), Expires: time.Now().Add(30 * time.Minute), Test: true}
	e.Store.Raise(a)
	return a
}

func (e *Engine) Run(ctx context.Context) {
	quakes := time.NewTicker(60 * time.Second)
	alerts := time.NewTicker(5 * time.Minute)
	defer quakes.Stop()
	defer alerts.Stop()
	go e.PollQuakes(ctx)
	go e.PollAlerts(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-quakes.C:
			go e.PollQuakes(ctx)
		case <-alerts.C:
			go e.PollAlerts(ctx)
		}
	}
}
