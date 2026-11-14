package alert

import (
	"testing"
	"time"
)

func TestHoldAndDismiss(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	changes := 0
	s := NewStore(func() time.Time { return now }, func() { changes++ })
	if !s.Raise(Alert{ID: "a", Kind: Earthquake, Severity: Severe}) {
		t.Fatal("first raise not reported as new")
	}
	if s.Raise(Alert{ID: "a", Kind: Earthquake, Severity: Severe}) {
		t.Fatal("identical raise reported as new")
	}
	if err := s.Dismiss("a"); err != ErrHeld {
		t.Fatalf("dismiss before hold = %v", err)
	}
	now = now.Add(HoldTime - time.Second)
	if err := s.Dismiss("a"); err != ErrHeld {
		t.Fatalf("dismiss one second early = %v", err)
	}
	now = now.Add(time.Second)
	if err := s.Dismiss("a"); err != nil {
		t.Fatalf("dismiss after hold = %v", err)
	}
	if s.Raise(Alert{ID: "a", Kind: Earthquake, Severity: Severe}) {
		t.Fatal("dismissed alert raised again")
	}
	if err := s.Dismiss("missing"); err != ErrUnknown {
		t.Fatalf("dismiss unknown = %v", err)
	}
	if changes != 2 {
		t.Fatalf("changes = %d", changes)
	}
}

func TestExpireKeepsHeldAlerts(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	s := NewStore(func() time.Time { return now }, nil)
	s.Raise(Alert{ID: "old", Severity: Extreme, Expires: now.Add(time.Minute)})
	s.Raise(Alert{ID: "big", Severity: Extreme})
	now = now.Add(2 * time.Minute)
	s.Expire()
	if len(s.Active()) != 2 {
		t.Fatal("expired alert removed while still held")
	}
	now = now.Add(4 * time.Minute)
	s.Expire()
	a := s.Active()
	if len(a) != 1 || a[0].ID != "big" {
		t.Fatalf("active after expiry = %+v", a)
	}
}

func TestOrder(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	s := NewStore(func() time.Time { return now }, nil)
	s.Raise(Alert{ID: "m", Severity: Moderate})
	now = now.Add(time.Second)
	s.Raise(Alert{ID: "x", Severity: Extreme})
	if a := s.Active(); a[0].ID != "x" {
		t.Fatalf("order = %+v", a)
	}
}

func TestDangerAndHold(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	s := NewStore(func() time.Time { return now }, nil)
	storm := Alert{ID: "storm", Kind: Storm, Severity: Severe}
	quake := Alert{ID: "quake", Kind: Earthquake, Severity: Severe}
	if storm.Danger() || !quake.Danger() || !(Alert{Kind: Storm, Severity: Extreme}).Danger() || (Alert{Kind: Earthquake, Severity: Moderate}).Danger() {
		t.Fatal("danger rule wrong")
	}
	s.Raise(storm)
	s.Raise(quake)
	if err := s.Dismiss("storm"); err != nil {
		t.Fatalf("a storm warning could not be dismissed at once: %v", err)
	}
	if err := s.Dismiss("quake"); err != ErrHeld {
		t.Fatalf("a dangerous alert was dismissed at once: %v", err)
	}
}
