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

func TestSetListMetricNames(t *testing.T) {
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

func TestSetUnregisterMetric(t *testing.T) {
	s := NewSet()
	// Initialize a few metrics
	for i := 0; i < 5; i++ {
		c := s.NewCounter(fmt.Sprintf("counter_%d", i))
		c.Inc()
		sm := s.NewSummary(fmt.Sprintf("summary_%d", i))
		sm.Update(float64(i))
	}
	// Unregister existing metrics
	if !s.UnregisterMetric("counter_1") {
		t.Fatalf("UnregisterMetric(counter_1) must return true")
	}
	if !s.UnregisterMetric("summary_1") {
		t.Fatalf("UnregisterMetric(summary_1) must return true")
	}

	// Unregister twice must return false
	if s.UnregisterMetric("counter_1") {
		t.Fatalf("UnregisterMetric(counter_1) must return false on unregistered metric")
	}
	if s.UnregisterMetric("summary_1") {
		t.Fatalf("UnregisterMetric(summary_1) must return false on unregistered metric")
	}

	// Validate metrics are removed
	const cName, smName = "counter_1", "summary_1"
	ok := false
	for _, n := range s.ListMetricNames() {
		if n == cName || n == smName {
			ok = true
		}
	}
	if ok {
		t.Fatalf("Metric counter_1 and summary_1 must not be listed anymore after unregister")
	}

	// re-register with the same names supposed
	// to be successful
	s.NewCounter(cName).Inc()
	s.NewSummary(smName).Update(float64(1))
}
