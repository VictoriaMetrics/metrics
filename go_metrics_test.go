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
	}, `foo_bucket{le="2"} 1
foo_bucket{le="3"} 3
foo_bucket{le="4"} 6
foo_bucket{le="+Inf"} 6
`)

	f(&runtimemetrics.Float64Histogram{
		Counts:  []uint64{0, 25, 1, 0},
		Buckets: []float64{1, 2, 3, 4, math.Inf(1)},
	}, `foo_bucket{le="2"} 0
foo_bucket{le="3"} 25
foo_bucket{le="4"} 26
foo_bucket{le="+Inf"} 26
`)

	f(&runtimemetrics.Float64Histogram{
		Counts:  []uint64{0, 25, 1, 3, 0, 44, 15, 132, 10, 0},
		Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, math.Inf(1)},
	}, `foo_bucket{le="2"} 0
foo_bucket{le="3"} 25
foo_bucket{le="4"} 26
foo_bucket{le="5"} 29
foo_bucket{le="6"} 29
foo_bucket{le="7"} 73
foo_bucket{le="8"} 88
foo_bucket{le="9"} 220
foo_bucket{le="10"} 230
foo_bucket{le="+Inf"} 230
`)

	f(&runtimemetrics.Float64Histogram{
		Counts:  []uint64{1, 5, 0},
		Buckets: []float64{math.Inf(-1), 4, 5, math.Inf(1)},
	}, `foo_bucket{le="4"} 1
foo_bucket{le="5"} 6
foo_bucket{le="+Inf"} 6
`)
}
