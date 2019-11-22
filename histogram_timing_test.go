package metrics

import (
	"testing"
)

func BenchmarkHistogramUpdate(b *testing.B) {
	h := GetOrCreateHistogram("BenchmarkHistogramUpdate")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			h.Update(float64(i))
			i++
		}
	})
}
