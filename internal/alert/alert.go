package alert

import (
	"errors"
	"sort"
	"sync"
	"time"
)

const HoldTime = 5 * time.Minute

var ErrHeld = errors.New("alert cannot be dismissed yet")
var ErrUnknown = errors.New("no such alert")

type Kind string

const (
	Earthquake Kind = "earthquake"
	Tsunami    Kind = "tsunami"
	Flood      Kind = "flood"
	Fire       Kind = "fire"
	Storm      Kind = "storm"
	Wind       Kind = "wind"
	Snow       Kind = "snow"
	Heat       Kind = "heat"
	Cold       Kind = "cold"
	Volcano    Kind = "volcano"
	Landslide  Kind = "landslide"
	Health     Kind = "health"
	Security   Kind = "security"
	National   Kind = "national"
	Other      Kind = "other"
)

type Severity int

const (
	Minor Severity = iota + 1
	Moderate
	Severe
	Extreme
)

func (s Severity) String() string {
	switch s {
	case Extreme:
		return "extreme"
	case Severe:
		return "severe"
	case Moderate:
		return "moderate"
	}
	return "minor"
}

type Alert struct {
	ID          string
	Kind        Kind
	Severity    Severity
	Headline    string
	Description string
	Instruction string
	Area        string
	Source      string
	Link        string
	Language    string
	Sent        time.Time
	Expires     time.Time
	Raised      time.Time
	Test        bool
}

func (a Alert) DismissableAt() time.Time {
	return a.Raised.Add(HoldTime)
}

type Store struct {
	mu        sync.Mutex
	active    map[string]Alert
	dismissed map[string]time.Time
	now       func() time.Time
	changed   func()
}

func NewStore(now func() time.Time, changed func()) *Store {
	if now == nil {
		now = time.Now
	}
	if changed == nil {
		changed = func() {}
	}
	return &Store{active: map[string]Alert{}, dismissed: map[string]time.Time{}, now: now, changed: changed}
}

func (s *Store) Raise(a Alert) bool {
	s.mu.Lock()
	if _, seen := s.dismissed[a.ID]; seen {
		s.mu.Unlock()
		return false
	}
	old, had := s.active[a.ID]
	if had {
		a.Raised = old.Raised
		if old == a {
			s.mu.Unlock()
			return false
		}
	} else {
		a.Raised = s.now()
	}
	s.active[a.ID] = a
	s.mu.Unlock()
	s.changed()
	return !had
}

func (s *Store) Dismiss(id string) error {
	s.mu.Lock()
	a, ok := s.active[id]
	if !ok {
		s.mu.Unlock()
		return ErrUnknown
	}
	if s.now().Before(a.DismissableAt()) {
		s.mu.Unlock()
		return ErrHeld
	}
	delete(s.active, id)
	s.dismissed[id] = s.now()
	s.mu.Unlock()
	s.changed()
	return nil
}

func (s *Store) Expire() {
	s.mu.Lock()
	now := s.now()
	n := 0
	for id, a := range s.active {
		if !a.Expires.IsZero() && now.After(a.Expires) && !now.Before(a.DismissableAt()) {
			delete(s.active, id)
			n++
		}
	}
	for id, at := range s.dismissed {
		if now.Sub(at) > 7*24*time.Hour {
			delete(s.dismissed, id)
		}
	}
	s.mu.Unlock()
	if n > 0 {
		s.changed()
	}
}

func (s *Store) Active() []Alert {
	s.mu.Lock()
	out := make([]Alert, 0, len(s.active))
	for _, a := range s.active {
		out = append(out, a)
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		return out[i].Raised.After(out[j].Raised)
	})
	return out
}
