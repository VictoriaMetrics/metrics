package metrics

import (
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

func TestNewSet(t *testing.T) {
	var ss []*Set
	for range 10 {
		s := NewSet()
		ss = append(ss, s)
	}
	for i := range 10 {
		s := ss[i]
		for j := range 10 {
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

func TestSetUnregisterAllMetrics(t *testing.T) {
	s := NewSet()
	for j := range 3 {
		expectedMetricsCount := 0
		for i := range 10 {
			_ = s.NewCounter(fmt.Sprintf("counter_%d", i))
			_ = s.NewSummary(fmt.Sprintf("summary_%d", i))
			_ = s.NewHistogram(fmt.Sprintf("histogram_%d", i))
			_ = s.NewGauge(fmt.Sprintf("gauge_%d", i), func() float64 { return 0 })
			expectedMetricsCount += 4
		}
		if mns := s.ListMetricNames(); len(mns) != expectedMetricsCount {
			t.Fatalf("unexpected number of metric names on iteration %d; got %d; want %d;\nmetric names:\n%q", j, len(mns), expectedMetricsCount, mns)
		}
		s.UnregisterAllMetrics()
		if mns := s.ListMetricNames(); len(mns) != 0 {
			t.Fatalf("unexpected metric names after UnregisterAllMetrics call on iteration %d: %q", j, mns)
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
	for range workers {
		go func() {
			defer wg.Done()
			now := time.Now()
			for i := range int(iterations) {
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

// Benchmark: MixedSet (realistic scenario with multiple metrics)
func BenchmarkMixedSet_WritePrometheus(b *testing.B) {
	s := NewSet()

	// 5 counters
	for i := 0; i < 5; i++ {
		c := s.NewCounter("benchmark_mixed_counter_total{instance=\"" + string(rune('a'+i)) + "\"}")
		c.Set(uint64(i * 1000))
	}

	// 3 float counters
	for i := 0; i < 3; i++ {
		fc := s.NewFloatCounter("benchmark_mixed_float_total{instance=\"" + string(rune('a'+i)) + "\"}")
		fc.Add(float64(i) * 1.5)
	}

	// 3 gauges
	for i := 0; i < 3; i++ {
		val := float64(i) * 10.5
		s.NewGauge("benchmark_mixed_gauge{instance=\""+string(rune('a'+i))+"\"}", func() float64 { return val })
	}

	// 2 histograms (vmrange)
	for i := 0; i < 2; i++ {
		h := s.NewHistogram("benchmark_mixed_histogram{instance=\"" + string(rune('a'+i)) + "\"}")
		for j := 0; j < 100; j++ {
			h.Update(float64(j) * 0.01)
		}
	}

	// 1 prometheus histogram (le-style)
	ph := s.NewPrometheusHistogram("benchmark_mixed_prom_histogram")
	for i := 0; i < 100; i++ {
		ph.Update(float64(i) * 0.01)
	}

	// 2 summaries
	for i := 0; i < 2; i++ {
		sm := s.NewSummary("benchmark_mixed_summary{instance=\"" + string(rune('a'+i)) + "\"}")
		for j := 0; j < 100; j++ {
			sm.Update(float64(j) * 0.001)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.WritePrometheus(io.Discard)
	}
}
