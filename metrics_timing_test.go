package metrics

import (
	"io"
	"testing"
	"time"
)

func BenchmarkWritePrometheus(b *testing.B) {

	f := func(b *testing.B, name string, setup func(*Set)) {
		b.Helper()
		b.Run(name, func(b *testing.B) {
			s := NewSet()
			defer func() { s.UnregisterAllMetrics() }()

			setup(s)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				s.WritePrometheus(io.Discard)
			}
		})
	}

	f(b, "counter", func(s *Set) {
		c := s.NewCounter("counter_total")
		c.Set(12345)
	})

	f(b, "float_counter", func(s *Set) {
		fc := s.NewFloatCounter("float_counter_total")
		fc.Add(123456.789)
	})

	f(b, "gauge", func(s *Set) {
		_ = s.NewGauge("gauge", func() float64 { return 42 })
	})

	f(b, "histogram", func(s *Set) {
		h := s.NewHistogram("histogram")
		for i := range 1000 {
			h.Update(float64(i) * 0.001)
		}
	})
	f(b, "histogram_prometheus", func(s *Set) {
		h := s.NewPrometheusHistogram("histogram_prometheus")
		for i := range 1000 {
			h.Update(float64(i) * 0.001)
		}
	})
	f(b, "histogram_prometheus_ext", func(s *Set) {
		buckets := ExponentialBuckets(0.001, 2, 20)
		h := s.NewPrometheusHistogramExt("histogram_prometheus_ext", buckets)
		for i := range 1000 {
			h.Update(float64(i) * 0.001)
		}
	})

	f(b, "summary", func(s *Set) {
		sm := s.NewSummary("summary")
		for i := range 1000 {
			sm.Update(float64(i) * 0.001)
		}
	})

	f(b, "summary_ext", func(s *Set) {
		quantiles := []float64{0.5, 0.9, 0.95, 0.99, 1.0}
		sm := s.NewSummaryExt("summary_ext", time.Second, quantiles)
		for i := range 1000 {
			sm.Update(float64(i) * 0.001)
		}
	})
}
