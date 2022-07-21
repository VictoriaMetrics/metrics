package metrics

import (
	"testing"
)

func BenchmarkAddExtraLabels(b *testing.B) {
	extraLabels := `foo="bar"`
	src := []byte(`foo 1
bar{baz="x"} 2
`)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		var dst []byte
		for pb.Next() {
			dst = addExtraLabels(dst[:0], src, extraLabels)
		}
	})
}
