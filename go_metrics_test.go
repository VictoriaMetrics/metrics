package metrics

import (
	"math"
	runtimemetrics "runtime/metrics"
	"strings"
	"testing"
)

func TestWriteRuntimeHistogramMetricOk(t *testing.T) {
	f := func(h *runtimemetrics.Float64Histogram, resultExpected string) {
		t.Helper()
		var wOut strings.Builder
		writeRuntimeHistogramMetric(&wOut, "foo", h)
		result := wOut.String()
		if result != resultExpected {
			t.Fatalf("unexpected result; got\n%s\nwant\n%s", result, resultExpected)
		}

	}

	f(&runtimemetrics.Float64Histogram{
		Counts:  []uint64{1, 2, 3},
		Buckets: []float64{1, 2, 3, 4},
	}, `foo{quantile="0.5"} 3
foo{quantile="0.9"} 4
foo{quantile="0.97"} 4
foo{quantile="0.99"} 4
foo{quantile="1"} 4
`)

	f(&runtimemetrics.Float64Histogram{
		Counts:  []uint64{0, 25, 1, 0},
		Buckets: []float64{1, 2, 3, 4, math.Inf(1)},
	}, `foo{quantile="0.5"} 3
foo{quantile="0.9"} 3
foo{quantile="0.97"} 4
foo{quantile="0.99"} 4
foo{quantile="1"} 4
`)

	f(&runtimemetrics.Float64Histogram{
		Counts:  []uint64{0, 25, 1, 3, 0, 44, 15, 132, 10, 0},
		Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, math.Inf(1)},
	}, `foo{quantile="0.5"} 9
foo{quantile="0.9"} 9
foo{quantile="0.97"} 10
foo{quantile="0.99"} 10
foo{quantile="1"} 10
`)

	f(&runtimemetrics.Float64Histogram{
		Counts:  []uint64{1, 5, 0},
		Buckets: []float64{math.Inf(-1), 4, 5, math.Inf(1)},
	}, `foo{quantile="0.5"} 5
foo{quantile="0.9"} 5
foo{quantile="0.97"} 5
foo{quantile="0.99"} 5
foo{quantile="1"} 5
`)
}
