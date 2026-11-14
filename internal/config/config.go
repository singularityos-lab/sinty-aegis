package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/singularityos-lab/sinty-aegis/internal/alert"
	"github.com/singularityos-lab/sinty-aegis/internal/geo"
)

type Contact struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

type Medical struct {
	Name       string `json:"name"`
	Blood      string `json:"blood"`
	Allergies  string `json:"allergies"`
	Conditions string `json:"conditions"`
	Medication string `json:"medication"`
	Notes      string `json:"notes"`
	OnLock     bool   `json:"on_lock"`
}

type Config struct {
	Enabled      bool      `json:"enabled"`
	Country      string    `json:"country"`
	Lat          float64   `json:"lat"`
	Lon          float64   `json:"lon"`
	Places       []string  `json:"places"`
	Languages    []string  `json:"languages"`
	MinSeverity  int       `json:"min_severity"`
	MinMagnitude float64   `json:"min_magnitude"`
	Muted        []string  `json:"muted"`
	Contacts     []Contact `json:"contacts"`
	Medical      Medical   `json:"medical"`
}

func Default() Config {
	return Config{Enabled: true, MinSeverity: int(alert.Moderate), MinMagnitude: 3.5}
}

func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "singularity", "aegis.json")
}

func Load(path string) Config {
	c := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	if json.Unmarshal(data, &c) != nil {
		return Default()
	}
	return c.Clean()
}

func (c Config) Clean() Config {
	c.Country = strings.ToLower(strings.TrimSpace(c.Country))
	if c.MinSeverity < int(alert.Minor) || c.MinSeverity > int(alert.Extreme) {
		c.MinSeverity = int(alert.Moderate)
	}
	if c.MinMagnitude < 2 || c.MinMagnitude > 9 {
		c.MinMagnitude = 3.5
	}
	if c.Lat < -90 || c.Lat > 90 || c.Lon < -180 || c.Lon > 180 {
		c.Lat, c.Lon = 0, 0
	}
	return c
}

func (c Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c Config) Where(zoneTab string) (string, geo.Point, bool) {
	country, point := c.Country, geo.Point{Lat: c.Lat, Lon: c.Lon}
	located := c.Lat != 0 || c.Lon != 0
	if country != "" && located {
		return country, point, true
	}
	zone, ok := geo.ZoneFor(geo.LocalTimezone(), zoneTab)
	if !ok {
		return country, point, country != "" && located
	}
	if country == "" {
		country = zone.Country
	}
	if !located {
		point = zone.Point
	}
	return country, point, true
}

func (c Config) Mutes(kind alert.Kind) bool {
	for _, m := range c.Muted {
		if m == string(kind) {
			return true
		}
	}
	return false
}
