package metrics

import (
	"fmt"
	"sync"
	"testing"
	"time"
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
	const cName, smName = "counter_1", "summary_1"
	// Initialize a few metrics
	c := s.NewCounter(cName)
	c.Inc()
	sm := s.NewSummary(smName)
	sm.Update(1)

	// Unregister existing metrics
	if !s.UnregisterMetric(cName) {
		t.Fatalf("UnregisterMetric(%s) must return true", cName)
	}
	if !s.UnregisterMetric(smName) {
		t.Fatalf("UnregisterMetric(%s) must return true", smName)
	}

	// Unregister twice must return false
	if s.UnregisterMetric(cName) {
		t.Fatalf("UnregisterMetric(%s) must return false on unregistered metric", cName)
	}
	if s.UnregisterMetric(smName) {
		t.Fatalf("UnregisterMetric(%s) must return false on unregistered metric", smName)
	}

	// verify that registry is empty
	if len(s.m) != 0 {
		t.Fatalf("expected metrics map to be empty; got %d elements", len(s.m))
	}
	if len(s.a) != 0 {
		t.Fatalf("expected metrics list to be empty; got %d elements", len(s.a))
	}

	// Validate metrics are removed
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

// TestRegisterUnregister tests concurrent access to
// metrics during registering and unregistering.
// Should be tested specifically with `-race` enabled.
func TestRegisterUnregister(t *testing.T) {
	const (
		workers    = 16
		iterations = 1e3
	)
	wg := sync.WaitGroup{}
	wg.Add(workers)
	for n := 0; n < workers; n++ {
		go func() {
			defer wg.Done()
			now := time.Now()
			for i := 0; i < iterations; i++ {
				iteration := i % 5
				counter := fmt.Sprintf(`counter{iteration="%d"}`, iteration)
				GetOrCreateCounter(counter).Add(i)
				UnregisterMetric(counter)

				histogram := fmt.Sprintf(`histogram{iteration="%d"}`, iteration)
				GetOrCreateHistogram(histogram).UpdateDuration(now)
				UnregisterMetric(histogram)

				gauge := fmt.Sprintf(`gauge{iteration="%d"}`, iteration)
				GetOrCreateGauge(gauge, func() float64 { return 1 })
				UnregisterMetric(gauge)

				summary := fmt.Sprintf(`summary{iteration="%d"}`, iteration)
				GetOrCreateSummary(summary).Update(float64(i))
				UnregisterMetric(summary)
			}
		}()
	}
	wg.Wait()
}
