package metrics

import (
	"fmt"
	"io"
	"math"
	"sync"
)

var defaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// OTLP Histogram is a histogram for non-negative values with pre-defined buckets
//
// Each bucket contains a counter for values in the given range.
// Each bucket is exposed via the following metric:
//
//	<metric_name>_bucket{<optional_tags>,le="upper_bound"} <counter>
//
// Where:
//
//   - <metric_name> is the metric name passed to NewHistogram
//   - <optional_tags> is optional tags for the <metric_name>, which are passed to NewHistogram
//   - <upper_bound> - upper bound of the current bucket. all samples <= upper_bound are in that bucket
//   - <counter> - the number of hits to the given bucket during Update* calls
//
// Zero histogram is usable.
type OTLPHistogram struct {
	// Mu gurantees synchronous update for all the counters and sum.
	//
	// Do not use sync.RWMutex, since it has zero sense from performance PoV.
	// It only complicates the code.
	mu sync.Mutex

	upperBounds []float64
	buckets     []uint64

	// count is the counter for all observations on this histogram
	count uint64

	// sum is the sum of all the values put into Histogram
	sum float64
}

// Reset resets the given histogram.
func (h *OTLPHistogram) Reset() {
	h.mu.Lock()
	for i := range h.buckets {
		h.buckets[i] = 0
	}

	h.sum = 0
	h.count = 0
	h.mu.Unlock()
}

// Update updates h with v.
//
// Negative values and NaNs are ignored.
func (h *OTLPHistogram) Update(v float64) {
	if math.IsNaN(v) || v < 0 {
		// Skip NaNs and negative values.
		return
	}
	bucketIdx := -1
	for i, ub := range h.upperBounds {
		if v <= ub {
			bucketIdx = i
			break
		}
	}
	h.mu.Lock()
	h.sum += v
	if bucketIdx == -1 {
		// +Inf, nothing to do
	}
	h.buckets[bucketIdx]++
	h.mu.Unlock()
}

// Merge merges src to h
func (h *OTLPHistogram) Merge(src *OTLPHistogram) {
	// first we must compare if the upper bounds are identical

	h.mu.Lock()
	defer h.mu.Unlock()

	src.mu.Lock()
	defer src.mu.Unlock()

	h.sum += src.sum
	h.count += src.count

	// TODO: implement actual sum
}

// NewOTLPHistogram creates and returns new OTLP histogram with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
func NewOTLPHistogram(name string) *OTLPHistogram {
	return defaultSet.NewOTLPHistogram(name)
}

// GetOrCreateHistogram returns registered histogram with the given name
// or creates new histogram if the registry doesn't contain histogram with
// the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
//
// Performance tip: prefer NewHistogram instead of GetOrCreateHistogram.
func GetOrCreateOTLPHistogram(name string) *OTLPHistogram {
	return defaultSet.GetOrCreateOTLPHistogram(name)
}

func (h *OTLPHistogram) marshalTo(prefix string, w io.Writer) {
	cumulativeSum := uint64(0)
	h.mu.Lock()
	for i, ub := range h.upperBounds {
		cumulativeSum += h.buckets[i]
		tag := fmt.Sprintf(`le="%v"`, ub)
		metricName := addTag(prefix, tag)
		name, labels := splitMetricName(metricName)
		fmt.Fprintf(w, "%s_bucket%s %d\n", name, labels, cumulativeSum)
	}
	count := h.count
	sum := h.sum
	h.mu.Unlock()

	tag := fmt.Sprintf("le=%q", "+Inf")
	metricName := addTag(prefix, tag)
	name, labels := splitMetricName(metricName)
	fmt.Fprintf(w, "%s_bucket%s %d\n", name, labels, count)

	name, labels = splitMetricName(prefix)
	if float64(int64(sum)) == sum {
		fmt.Fprintf(w, "%s_sum%s %d\n", name, labels, int64(sum))
	} else {
		fmt.Fprintf(w, "%s_sum%s %g\n", name, labels, sum)
	}
	fmt.Fprintf(w, "%s_count%s %d\n", name, labels, count)
}

func (h *OTLPHistogram) metricType() string {
	return "histogram"
}
