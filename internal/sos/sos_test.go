package sos

import "testing"

func TestNumbers(t *testing.T) {
	cases := map[string]string{"it": "112", "US": "911", "gb": "999", "au": "000", "nz": "111", "jp": "110", "de": "112"}
	for country, first := range cases {
		n := Numbers(country)
		if len(n) == 0 || n[0] != first {
			t.Fatalf("%s numbers = %v", country, n)
		}
	}
	if n := Numbers("zz"); len(n) != 1 || n[0] != "112" {
		t.Fatalf("unknown country numbers = %v", n)
	}
	if !Known("it", "118") || !Known("us", "112") || Known("it", "3331234567") || Known("us", "999") {
		t.Fatal("emergency number check wrong")
	}
	n := Numbers("it")
	n[0] = "000"
	if Numbers("it")[0] != "112" {
		t.Fatal("table mutated through the returned slice")
	}
}
