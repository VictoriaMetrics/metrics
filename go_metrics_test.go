package metrics

import (
	"math"
	runtime_metrics "runtime/metrics"
	"strings"
	"testing"
)

func TestWriteRuntimeHistogramMetricOk(t *testing.T) {
	f := func(expected string, metricName string, h runtime_metrics.Float64Histogram) {
		t.Helper()
		var wOut strings.Builder
		writeRuntimeHistogramMetric(&wOut, metricName, &h)
		got := wOut.String()
		if got != expected {
			t.Fatalf("got out: \n%s\nwant: \n%s", got, expected)
		}

	}

	f(`runtime_latency_seconds{quantile="0"} 1
runtime_latency_seconds{quantile="0.25"} 3
runtime_latency_seconds{quantile="0.5"} 4
runtime_latency_seconds{quantile="0.75"} 4
runtime_latency_seconds{quantile="0.95"} 4
runtime_latency_seconds{quantile="1"} 4
`,
		`runtime_latency_seconds`, runtime_metrics.Float64Histogram{
			Counts:  []uint64{1, 2, 3},
			Buckets: []float64{1.0, 2.0, 3.0, 4.0},
		})
	f(`runtime_latency_seconds{quantile="0"} 1
runtime_latency_seconds{quantile="0.25"} 3
runtime_latency_seconds{quantile="0.5"} 3
runtime_latency_seconds{quantile="0.75"} 3
runtime_latency_seconds{quantile="0.95"} 4
runtime_latency_seconds{quantile="1"} 4
`,
		`runtime_latency_seconds`, runtime_metrics.Float64Histogram{
			Counts:  []uint64{0, 25, 1, 3, 0},
			Buckets: []float64{1.0, 2.0, 3.0, 4.0, math.Inf(1)},
		})
	f(`runtime_latency_seconds{quantile="0"} 1
runtime_latency_seconds{quantile="0.25"} 7
runtime_latency_seconds{quantile="0.5"} 9
runtime_latency_seconds{quantile="0.75"} 9
runtime_latency_seconds{quantile="0.95"} 10
runtime_latency_seconds{quantile="1"} 10
`,
		`runtime_latency_seconds`, runtime_metrics.Float64Histogram{
			Counts:  []uint64{0, 25, 1, 3, 0, 44, 15, 132, 10, 11},
			Buckets: []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0, math.Inf(1)},
		})
	f(`runtime_latency_seconds{quantile="0"} -Inf
runtime_latency_seconds{quantile="0.25"} 4
runtime_latency_seconds{quantile="0.5"} 4
runtime_latency_seconds{quantile="0.75"} 4
runtime_latency_seconds{quantile="0.95"} 4
runtime_latency_seconds{quantile="1"} 4
`,
		`runtime_latency_seconds`, runtime_metrics.Float64Histogram{
			Counts:  []uint64{1, 5},
			Buckets: []float64{math.Inf(-1), 4.0, math.Inf(1)},
		})
}

func TestWriteRuntimeHistogramMetricFail(t *testing.T) {
	f := func(h runtime_metrics.Float64Histogram) {
		t.Helper()
		var wOut strings.Builder
		writeRuntimeHistogramMetric(&wOut, ``, &h)
		got := wOut.String()
		if got != "" {
			t.Fatalf("expected empty output, got out: \n%s", got)
		}

	}

	f(runtime_metrics.Float64Histogram{
		Counts:  []uint64{},
		Buckets: []float64{},
	})
	f(runtime_metrics.Float64Histogram{
		Counts:  []uint64{0, 25, 1, 3, 0, 12, 12},
		Buckets: []float64{1.0, 2.0, 3.0, 4.0, math.Inf(1)},
	})
}
