package registry

import (
	"os"
	"testing"
)

func TestRegistry(t *testing.T) {
	data, err := os.ReadFile("testdata/sources.json")
	if err != nil {
		t.Fatal(err)
	}
	all, err := Parse(data)
	if err != nil || len(all) < 200 {
		t.Fatalf("sources = %d err = %v", len(all), err)
	}
	it := For(all, "IT", []string{"it", "en"})
	if len(it) != 1 || it[0].ID != "it-meteoam-it" || it[0].Feed != "https://feeds.meteoalarm.org/feeds/meteoalarm-legacy-atom-italy" {
		t.Fatalf("Italy = %+v", it)
	}
	de := For(all, "de", []string{"en"})
	if len(de) != 1 || de[0].ID != "de-dwd-en" {
		t.Fatalf("Germany in English = %+v", de)
	}
	de = For(all, "de", []string{"it"})
	if len(de) != 1 || de[0].ID != "de-dwd-de" {
		t.Fatalf("Germany without a matching language = %+v", de)
	}
	us := For(all, "us", []string{"en"})
	if len(us) < 3 {
		t.Fatalf("United States = %+v", us)
	}
	for _, s := range us {
		if !s.Official {
			t.Fatalf("unofficial source kept: %+v", s)
		}
	}
	if len(For(all, "zz", nil)) != 0 {
		t.Fatal("unknown country has sources")
	}
}
