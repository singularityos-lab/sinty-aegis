package registry

import (
	"encoding/json"
	"strings"
)

const URL = "https://alert-hub-sources.s3.amazonaws.com/json"

type Source struct {
	ID       string
	Name     string
	Country  string
	Language string
	Feed     string
	Official bool
}

type doc struct {
	Sources []struct {
		Source struct {
			ID         string `json:"sourceId"`
			ByLanguage []struct {
				Code string `json:"code"`
				Name string `json:"name"`
			} `json:"byLanguage"`
			Official bool   `json:"sourceIsOfficial"`
			Feed     string `json:"capAlertFeed"`
			Status   string `json:"capAlertFeedStatus"`
			Country  string `json:"authorityCountry"`
		} `json:"source"`
	} `json:"sources"`
}

func Parse(data []byte) ([]Source, error) {
	var d doc
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	var out []Source
	for _, e := range d.Sources {
		s := e.Source
		if s.Feed == "" || s.Status != "operating" || !strings.HasPrefix(s.Feed, "https://") {
			continue
		}
		src := Source{ID: s.ID, Country: strings.ToLower(s.Country), Feed: s.Feed, Official: s.Official}
		if len(s.ByLanguage) > 0 {
			src.Language = strings.ToLower(s.ByLanguage[0].Code)
			src.Name = s.ByLanguage[0].Name
			if _, rest, ok := strings.Cut(src.Name, ": "); ok {
				src.Name = rest
			}
		}
		out = append(out, src)
	}
	return out, nil
}

func For(all []Source, country string, languages []string) []Source {
	country = strings.ToLower(country)
	byAuthority := map[string][]Source{}
	var order []string
	for _, s := range all {
		if s.Country != country || !s.Official {
			continue
		}
		key := s.ID
		if i := strings.LastIndex(key, "-"); i > 0 {
			key = key[:i]
		}
		if _, seen := byAuthority[key]; !seen {
			order = append(order, key)
		}
		byAuthority[key] = append(byAuthority[key], s)
	}
	var out []Source
	for _, key := range order {
		group := byAuthority[key]
		pick := group[0]
	search:
		for _, want := range languages {
			for _, s := range group {
				if s.Language == strings.ToLower(want) {
					pick = s
					break search
				}
			}
		}
		out = append(out, pick)
	}
	return out
}
