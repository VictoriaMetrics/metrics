package metrics

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
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

	sum   float64
	count uint64

	window time.Duration
}

// NewSummary creates and returns new summary with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
func NewSummary(name string) *Summary {
	return defaultSet.NewSummary(name)
}

// NewSummaryExt creates and returns new summary with the given name,
// window and quantiles.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
func NewSummaryExt(name string, window time.Duration, quantiles []float64) *Summary {
	return defaultSet.NewSummaryExt(name, window, quantiles)
}

func newSummary(window time.Duration, quantiles []float64) *Summary {
	// Make a copy of quantiles in order to prevent from their modification by the caller.
	quantiles = append([]float64{}, quantiles...)
	validateQuantiles(quantiles)
	sm := &Summary{
		curr:           histogram.NewFast(),
		next:           histogram.NewFast(),
		quantiles:      quantiles,
		quantileValues: make([]float64, len(quantiles)),
		window:         window,
	}
	return sm
}

func validateQuantiles(quantiles []float64) {
	for _, q := range quantiles {
		if q < 0 || q > 1 {
			panic(fmt.Errorf("BUG: quantile must be in the range [0..1]; got %v", q))
		}
	}
}

// Update updates the summary.
func (sm *Summary) Update(v float64) {
	sm.mu.Lock()
	sm.curr.Update(v)
	sm.next.Update(v)
	sm.sum += v
	sm.count++
	sm.mu.Unlock()
}

// UpdateDuration updates request duration based on the given startTime.
func (sm *Summary) UpdateDuration(startTime time.Time) {
	d := time.Since(startTime).Seconds()
	sm.Update(d)
}

func (sm *Summary) marshalTo(prefix string, w *bytes.Buffer) {
	// Marshal only *_sum and *_count values.
	// Quantile values should be already updated by the caller via sm.updateQuantiles() call.
	// sm.quantileValues will be marshaled later via quantileValue.marshalTo.
	sm.mu.Lock()
	sum := sm.sum
	count := sm.count
	sm.mu.Unlock()

	if count > 0 {
		name, filters := splitMetricName(prefix)
		w.WriteString(name)
		w.WriteString("_sum")
		w.WriteString(filters)
		w.WriteByte(' ')
		if float64(int64(sum)) == sum {
			// Marshal integer sum without scientific notation
			b := strconv.AppendInt(w.AvailableBuffer(), int64(sum), 10)
			w.Write(b)
		} else {
			b := strconv.AppendFloat(w.AvailableBuffer(), sum, 'g', -1, 64)
			w.Write(b)
		}
		w.WriteByte('\n')
		w.WriteString(name)
		w.WriteString("_count")
		w.WriteString(filters)
		w.WriteByte(' ')
		b := strconv.AppendUint(w.AvailableBuffer(), count, 10)
		w.Write(b)
		w.WriteByte('\n')
	}
}

func (sm *Summary) metricType() string {
	// this metric type should not be printed, because summary (sum and count)
	// of the same metric family will be printed after quantile(s).
	// If metadata is needed, the metadata from quantile(s) should be used.
	// quantile will be printed first, so its metrics type won't be printed as metadata.
	// Printing quantiles before sum and count aligns this code with Prometheus behavior.
	// See: https://github.com/VictoriaMetrics/metrics/pull/99
	return "unsupported"
}

func splitMetricName(name string) (string, string) {
	n := strings.IndexByte(name, '{')
	if n < 0 {
		return name, ""
	}
	return name[:n], name[n:]
}

func (sm *Summary) updateQuantiles() {
	sm.mu.Lock()
	sm.quantileValues = sm.curr.Quantiles(sm.quantileValues[:0], sm.quantiles)
	sm.mu.Unlock()
}

// GetOrCreateSummary returns registered summary with the given name
// or creates new summary if the registry doesn't contain summary with
// the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewSummary instead of GetOrCreateSummary.
func GetOrCreateSummary(name string) *Summary {
	return defaultSet.GetOrCreateSummary(name)
}

// GetOrCreateSummaryExt returns registered summary with the given name,
// window and quantiles or creates new summary if the registry doesn't
// contain summary with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned summary is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewSummaryExt instead of GetOrCreateSummaryExt.
func GetOrCreateSummaryExt(name string, window time.Duration, quantiles []float64) *Summary {
	return defaultSet.GetOrCreateSummaryExt(name, window, quantiles)
}

func isEqualQuantiles(a, b []float64) bool {
	// Do not use relfect.DeepEqual, since it is slower than the direct comparison.
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type quantileValue struct {
	sm  *Summary
	idx int
}

func (qv *quantileValue) marshalTo(prefix string, w *bytes.Buffer) {
	qv.sm.mu.Lock()
	v := qv.sm.quantileValues[qv.idx]
	qv.sm.mu.Unlock()
	if !math.IsNaN(v) {
		w.WriteString(prefix)
		w.WriteByte(' ')
		b := strconv.AppendFloat(w.AvailableBuffer(), v, 'g', -1, 64)
		w.Write(b)
		w.WriteByte('\n')
	}
}

func (qv *quantileValue) metricType() string {
	return "summary"
}

func addTag(name, tag string) string {
	if len(name) == 0 || name[len(name)-1] != '}' {
		return fmt.Sprintf("%s{%s}", name, tag)
	}
	name = name[:len(name)-1]
	if len(name) == 0 {
		panic(fmt.Errorf("BUG: metric name cannot be empty"))
	}
	if name[len(name)-1] == '{' {
		// case for empty labels set metric_name{}
		return fmt.Sprintf("%s%s}", name, tag)
	}
	return fmt.Sprintf("%s,%s}", name, tag)
}

func registerSummaryLocked(sm *Summary) {
	window := sm.window
	summariesLock.Lock()
	summaries[window] = append(summaries[window], sm)
	if len(summaries[window]) == 1 {
		go summariesSwapCron(window)
	}
	summariesLock.Unlock()
}

func unregisterSummary(sm *Summary) {
	window := sm.window
	summariesLock.Lock()
	sms := summaries[window]
	found := false
	for i, xsm := range sms {
		if xsm == sm {
			sms = append(sms[:i], sms[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		panic(fmt.Errorf("BUG: cannot find registered summary %p", sm))
	}
	summaries[window] = sms
	summariesLock.Unlock()
}

func summariesSwapCron(window time.Duration) {
	for {
		time.Sleep(window / 2)
		summariesLock.Lock()
		for _, sm := range summaries[window] {
			sm.mu.Lock()
			tmp := sm.curr
			sm.curr = sm.next
			sm.next = tmp
			sm.next.Reset()
			sm.mu.Unlock()
		}
		summariesLock.Unlock()
	}
}

var (
	summaries     = map[time.Duration][]*Summary{}
	summariesLock sync.Mutex
)
