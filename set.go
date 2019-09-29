package metrics

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

// Set is a set of metrics.
//
// Metrics belonging to a set are exported separately from global metrics.
//
// Set.WritePrometheus must be called for exporting metrics from the set.
type Set struct {
	mu        sync.Mutex
	a         []*namedMetric
	m         map[string]*namedMetric
	summaries []*Summary
}

// NewSet creates new set of metrics.
func NewSet() *Set {
	return &Set{
		m: make(map[string]*namedMetric),
	}
}

// WritePrometheus writes all the metrics from s to w in Prometheus format.
func (s *Set) WritePrometheus(w io.Writer) {
	lessFunc := func(i, j int) bool {
		return s.a[i].name < s.a[j].name
	}
	s.mu.Lock()
	for _, sm := range s.summaries {
		sm.updateQuantiles()
	}
	if !sort.SliceIsSorted(s.a, lessFunc) {
		sort.Slice(s.a, lessFunc)
	}
	for _, nm := range s.a {
		nm.metric.marshalTo(nm.name, w)
	}
	s.mu.Unlock()
}

// NewCounter registers and returns new counter with the given name in the s.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned counter is safe to use from concurrent goroutines.
func (s *Set) NewCounter(name string) *Counter {
	c := &Counter{}
	s.registerMetric(name, c)
	return c
}

// GetOrCreateCounter returns registered counter in s with the given name
// or creates new counter if s doesn't contain counter with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned counter is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewCounter instead of GetOrCreateCounter.
func (s *Set) GetOrCreateCounter(name string) *Counter {
	s.mu.Lock()
	nm := s.m[name]
	s.mu.Unlock()
	if nm == nil {
		// Slow path - create and register missing counter.
		if err := validateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		nmNew := &namedMetric{
			name:   name,
			metric: &Counter{},
		}
		s.mu.Lock()
		nm = s.m[name]
		if nm == nil {
			nm = nmNew
			s.m[name] = nm
			s.a = append(s.a, nm)
		}
		s.mu.Unlock()
	}
	c, ok := nm.metric.(*Counter)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a Counter. It is %T", name, nm.metric))
	}
	return c
}

// NewGauge registers and returns gauge with the given name in s, which calls f
// to obtain gauge value.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// f must be safe for concurrent calls.
//
// The returned gauge is safe to use from concurrent goroutines.
func (s *Set) NewGauge(name string, f func() float64) *Gauge {
	if f == nil {
		panic(fmt.Errorf("BUG: f cannot be nil"))
	}
	g := &Gauge{
		f: f,
	}
	s.registerMetric(name, g)
	return g
}

// GetOrCreateGauge returns registered gauge with the given name in s
// or creates new gauge if s doesn't contain gauge with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned gauge is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewGauge instead of GetOrCreateGauge.
func (s *Set) GetOrCreateGauge(name string, f func() float64) *Gauge {
	s.mu.Lock()
	nm := s.m[name]
	s.mu.Unlock()
	if nm == nil {
		// Slow path - create and register missing gauge.
		if f == nil {
			panic(fmt.Errorf("BUG: f cannot be nil"))
		}
		if err := validateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		nmNew := &namedMetric{
			name: name,
			metric: &Gauge{
				f: f,
			},
		}
		s.mu.Lock()
		nm = s.m[name]
		if nm == nil {
			nm = nmNew
			s.m[name] = nm
			s.a = append(s.a, nm)
		}
		s.mu.Unlock()
	}
	g, ok := nm.metric.(*Gauge)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a Gauge. It is %T", name, nm.metric))
	}
	return g
}

// NewSummary creates and returns new summary with the given name in s.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
func (s *Set) NewSummary(name string) *Summary {
	return s.NewSummaryExt(name, defaultSummaryWindow, defaultSummaryQuantiles)
}

// NewSummaryExt creates and returns new summary in s with the given name,
// window and quantiles.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
func (s *Set) NewSummaryExt(name string, window time.Duration, quantiles []float64) *Summary {
	sm := newSummary(window, quantiles)
	s.registerMetric(name, sm)
	registerSummary(sm)
	s.registerSummaryQuantiles(name, sm)
	s.mu.Lock()
	s.summaries = append(s.summaries, sm)
	s.mu.Unlock()
	return sm
}

// GetOrCreateSummary returns registered summary with the given name in s
// or creates new summary if s doesn't contain summary with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewSummary instead of GetOrCreateSummary.
func (s *Set) GetOrCreateSummary(name string) *Summary {
	return s.GetOrCreateSummaryExt(name, defaultSummaryWindow, defaultSummaryQuantiles)
}

// GetOrCreateSummaryExt returns registered summary with the given name,
// window and quantiles in s or creates new summary if s doesn't
// contain summary with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewSummaryExt instead of GetOrCreateSummaryExt.
func (s *Set) GetOrCreateSummaryExt(name string, window time.Duration, quantiles []float64) *Summary {
	s.mu.Lock()
	nm := s.m[name]
	s.mu.Unlock()
	if nm == nil {
		// Slow path - create and register missing summary.
		if err := validateMetric(name); err != nil {
			panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
		}
		sm := newSummary(window, quantiles)
		nmNew := &namedMetric{
			name:   name,
			metric: sm,
		}
		mustRegisterQuantiles := false
		s.mu.Lock()
		nm = s.m[name]
		if nm == nil {
			nm = nmNew
			s.m[name] = nm
			s.a = append(s.a, nm)
			registerSummary(sm)
			mustRegisterQuantiles = true
		}
		s.summaries = append(s.summaries, sm)
		s.mu.Unlock()
		if mustRegisterQuantiles {
			s.registerSummaryQuantiles(name, sm)
		}
	}
	sm, ok := nm.metric.(*Summary)
	if !ok {
		panic(fmt.Errorf("BUG: metric %q isn't a Summary. It is %T", name, nm.metric))
	}
	if sm.window != window {
		panic(fmt.Errorf("BUG: invalid window requested for the summary %q; requested %s; need %s", name, window, sm.window))
	}
	if !isEqualQuantiles(sm.quantiles, quantiles) {
		panic(fmt.Errorf("BUG: invalid quantiles requested from the summary %q; requested %v; need %v", name, quantiles, sm.quantiles))
	}
	return sm
}

func (s *Set) registerSummaryQuantiles(name string, sm *Summary) {
	for i, q := range sm.quantiles {
		quantileValueName := addTag(name, fmt.Sprintf(`quantile="%g"`, q))
		qv := &quantileValue{
			sm:  sm,
			idx: i,
		}
		s.registerMetric(quantileValueName, qv)
	}
}

func (s *Set) registerMetric(name string, m metric) {
	if err := validateMetric(name); err != nil {
		panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
	}
	s.mu.Lock()
	nm, ok := s.m[name]
	if !ok {
		nm = &namedMetric{
			name:   name,
			metric: m,
		}
		s.m[name] = nm
		s.a = append(s.a, nm)
	}
	s.mu.Unlock()
	if ok {
		panic(fmt.Errorf("BUG: metric %q is already registered", name))
	}
}
