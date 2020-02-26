package metrics

import (
	"fmt"
	"testing"
)

func TestNewSet(t *testing.T) {
	var ss []*Set
	for i := 0; i < 10; i++ {
		s := NewSet()
		ss = append(ss, s)
	}
	for i := 0; i < 10; i++ {
		s := ss[i]
		for j := 0; j < 10; j++ {
			c := s.NewCounter(fmt.Sprintf("counter_%d", j))
			c.Inc()
			if n := c.Get(); n != 1 {
				t.Fatalf("unexpected counter value; got %d; want %d", n, 1)
			}
			g := s.NewGauge(fmt.Sprintf("gauge_%d", j), func() float64 { return 123 })
			if v := g.Get(); v != 123 {
				t.Fatalf("unexpected gauge value; got %v; want %v", v, 123)
			}
			sm := s.NewSummary(fmt.Sprintf("summary_%d", j))
			if sm == nil {
				t.Fatalf("NewSummary returned nil")
			}
			h := s.NewHistogram(fmt.Sprintf("histogram_%d", j))
			if h == nil {
				t.Fatalf("NewHistogram returned nil")
			}
		}
	}
}

func TestListMetricNames(t *testing.T) {
	s := NewSet()
	expect := []string{"cnt1", "cnt2", "cnt3"}
	// Initialize a few counters
	for _, n := range expect {
		c := s.NewCounter(n)
		c.Inc()
	}

	list := s.ListMetricNames()

	if len(list) != len(expect) {
		t.Fatalf("Metrics count is wrong for listing")
	}
	for _, e := range expect {
		found := false
		for _, n := range list {
			if e == n {
				found = true
			}
		}
		if !found {
			t.Fatalf("Metric %s not found in listing", e)
		}
	}
}

func TestUnregisterMetric(t *testing.T) {
	s := NewSet()
	// Initialize a few counters
	for i := 0; i < 5; i++ {
		c := s.NewCounter(fmt.Sprintf("counter_%d", i))
		c.Inc()
	}
	// Unregister existing counters
	ok := s.UnregisterMetric("counter_1")
	if !ok {
		t.Fatalf("Metric counter_1 should return true for deregistering")
	}

	// Unregister twice must return false
	ok = s.UnregisterMetric("counter_1")
	if ok {
		t.Fatalf("Metric counter_1 should not return false on unregister twice")
	}

	// Validate counters are removed
	ok = false
	for _, n := range s.ListMetricNames() {
		if n == "counter_1" {
			ok = true
		}
	}
	if ok {
		t.Fatalf("Metric counter_1 and counter_3 must not be listed anymore after unregister")
	}
}
