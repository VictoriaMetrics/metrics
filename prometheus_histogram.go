package metrics

import (
	"fmt"
	"io"
	"math"
	"sync"
)

var defaultUpperBounds = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// Prometheus Histogram is a histogram for non-negative values with pre-defined buckets
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
type PrometheusHistogram struct {
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
func (h *PrometheusHistogram) Reset() {
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
func (h *PrometheusHistogram) Update(v float64) {
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
	defer h.mu.Unlock()
	h.sum += v
	h.count++
	if bucketIdx == -1 {
		// +Inf, nothing to do, already accounted for in the total sum
		return
	}
	h.buckets[bucketIdx]++
}

// Merge merges src to h
func (h *PrometheusHistogram) Merge(src *PrometheusHistogram) {
	// first we must compare if the upper bounds are identical

	h.mu.Lock()
	defer h.mu.Unlock()

	src.mu.Lock()
	defer src.mu.Unlock()

	h.sum += src.sum
	h.count += src.count

	// TODO: implement actual sum
}

// NewPrometheusHistogram creates and returns new prometheus histogram with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
func NewPrometheusHistogram(name string) *PrometheusHistogram {
	return defaultSet.NewPrometheusHistogram(name)
}

// NewPrometheusHistogram creates and returns new prometheus histogram with the given name.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//   - foo
//   - foo{bar="baz"}
//   - foo{bar="baz",aaa="b"}
//
// The returned histogram is safe to use from concurrent goroutines.
func NewPrometheusHistogramExt(name string, buckets []float64) *PrometheusHistogram {
	return defaultSet.NewPrometheusHistogramExt(name, buckets)
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
func GetOrCreatePrometheusHistogram(name string) *PrometheusHistogram {
	return defaultSet.GetOrCreatePrometheusHistogram(name)
}

// GetOrCreateHistogramExt returns registered histogram with the given name and
// buckets or creates new histogram if the registry doesn't contain histogram
// with the given name.
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
func GetOrCreatePrometheusHistogramExt(name string, buckets []float64) *PrometheusHistogram {
	return defaultSet.GetOrCreatePrometheusHistogramExt(name, buckets)
}

func newPrometheusHistogram(upperBounds []float64) *PrometheusHistogram {
	validateBuckets(upperBounds)
	last := len(upperBounds) - 1
	if math.IsInf(upperBounds[last], +1) {
		upperBounds = upperBounds[:last] // ignore +Inf bucket as it is covered anyways
	}
	h := PrometheusHistogram{
		upperBounds: upperBounds,
		buckets:     make([]uint64, len(upperBounds)),
	}

	return &h
}

func validateBuckets(upperBounds []float64) {
	if len(upperBounds) == 0 {
		panic("no upper bounds were given for the buckets")
	}
	for i := 0; i < len(upperBounds)-1; i++ {
		if upperBounds[i] >= upperBounds[i+1] {
			panic("upper bounds for the buckets must be strictly increasing")
		}
	}
}

func (h *PrometheusHistogram) marshalTo(prefix string, w io.Writer) {
	cumulativeSum := uint64(0)
	h.mu.Lock()
	count := h.count
	sum := h.sum
	if count == 0 {
		h.mu.Unlock()
		return
	}
	for i, ub := range h.upperBounds {
		cumulativeSum += h.buckets[i]
		tag := fmt.Sprintf(`le="%v"`, ub)
		metricName := addTag(prefix, tag)
		name, labels := splitMetricName(metricName)
		fmt.Fprintf(w, "%s_bucket%s %d\n", name, labels, cumulativeSum)
	}
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

func (h *PrometheusHistogram) metricType() string {
	return "histogram"
}
