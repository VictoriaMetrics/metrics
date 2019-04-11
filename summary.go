package metrics

import (
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/valyala/histogram"
)

const defaultSummaryWindow = 5 * time.Minute

var defaultSummaryQuantiles = []float64{0.5, 0.9, 0.97, 0.99, 1}

// Summary implements summary.
type Summary struct {
	mu sync.Mutex

	curr *histogram.Fast
	next *histogram.Fast

	quantiles      []float64
	quantileValues []float64
}

// NewSummary creates and returns new summary with the given name.
//
// name must be valid Prometheus-compatible metric with possible lables.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
func NewSummary(name string) *Summary {
	return NewSummaryExt(name, defaultSummaryWindow, defaultSummaryQuantiles)
}

// NewSummaryExt creates and returns new summary with the given name,
// window and quantiles.
//
// name must be valid Prometheus-compatible metric with possible lables.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
func NewSummaryExt(name string, window time.Duration, quantiles []float64) *Summary {
	validateQuantiles(quantiles)
	s := &Summary{
		curr:           histogram.NewFast(),
		next:           histogram.NewFast(),
		quantiles:      quantiles,
		quantileValues: make([]float64, len(quantiles)),
	}
	registerSummary(s, window)
	registerMetric(name, s)
	for i, q := range quantiles {
		quantileValueName := addTag(name, fmt.Sprintf(`quantile="%g"`, q))
		qv := &quantileValue{
			s:   s,
			idx: i,
		}
		registerMetric(quantileValueName, qv)
	}
	return s
}

func validateQuantiles(quantiles []float64) {
	for _, q := range quantiles {
		if q < 0 || q > 1 {
			panic(fmt.Errorf("BUG: quantile must be in the range [0..1]; got %v", q))
		}
	}
}

// Update updates the summary.
func (s *Summary) Update(v float64) {
	s.mu.Lock()
	s.curr.Update(v)
	s.next.Update(v)
	s.mu.Unlock()
}

// UpdateDuration updates request duration based on the given startTime.
func (s *Summary) UpdateDuration(startTime time.Time) {
	d := time.Since(startTime).Seconds()
	s.Update(d)
}

func (s *Summary) marshalTo(prefix string, w io.Writer) {
	// Just update s.quantileValues and don't write anything to w.
	// s.quantileValues will be marshaled later via quantileValue.marshalTo.
	s.updateQuantiles()
}

func (s *Summary) updateQuantiles() {
	s.mu.Lock()
	s.quantileValues = s.curr.Quantiles(s.quantileValues[:0], s.quantiles)
	s.mu.Unlock()
}

type quantileValue struct {
	s   *Summary
	idx int
}

func (qv *quantileValue) marshalTo(prefix string, w io.Writer) {
	qv.s.mu.Lock()
	v := qv.s.quantileValues[qv.idx]
	qv.s.mu.Unlock()
	if !math.IsNaN(v) {
		fmt.Fprintf(w, "%s %g\n", prefix, v)
	}
}

func addTag(name, tag string) string {
	if len(name) == 0 || name[len(name)-1] != '}' {
		return fmt.Sprintf("%s{%s}", name, tag)
	}
	return fmt.Sprintf("%s,%s}", name[:len(name)-1], tag)
}

func registerSummary(s *Summary, window time.Duration) {
	summariesLock.Lock()
	summaries[window] = append(summaries[window], s)
	if len(summaries[window]) == 1 {
		go summariesSwapCron(window)
	}
	summariesLock.Unlock()
}

func summariesSwapCron(window time.Duration) {
	for {
		time.Sleep(window / 2)
		summariesLock.Lock()
		for _, s := range summaries[window] {
			s.mu.Lock()
			tmp := s.curr
			s.curr = s.next
			s.next = tmp
			s.next.Reset()
			s.mu.Unlock()
		}
		summariesLock.Unlock()
	}
}

var (
	summaries     = map[time.Duration][]*Summary{}
	summariesLock sync.Mutex
)
