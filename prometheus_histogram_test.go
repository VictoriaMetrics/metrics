package metrics

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrometheusHistogramSerial(t *testing.T) {
	name := "TestPrometheusHistogramSerial"
	h := NewPrometheusHistogram(name)

	// Verify that the histogram is invisible in the output of WritePrometheus when it has no data.
	var bb bytes.Buffer
	WritePrometheus(&bb, false)
	result := bb.String()
	if strings.Contains(result, name) {
		t.Fatalf("histogram %s shouldn't be visible in the WritePrometheus output; got\n%s", name, result)
	}

	// Write data to histogram
	for i := 98; i < 218; i++ {
		h.Update(float64(i)*1e-4)
	}

	// Make sure the histogram prints <prefix>_bucket on marshalTo call
	testMarshalTo(t, h, "prefix", `prefix_bucket{le="0.005"} 0
prefix_bucket{le="0.01"} 3
prefix_bucket{le="0.025"} 120
prefix_bucket{le="0.05"} 120
prefix_bucket{le="0.1"} 120
prefix_bucket{le="0.25"} 120
prefix_bucket{le="0.5"} 120
prefix_bucket{le="1"} 120
prefix_bucket{le="2.5"} 120
prefix_bucket{le="5"} 120
prefix_bucket{le="10"} 120
prefix_bucket{le="+Inf"} 120
prefix_sum 1.8900000000000003
prefix_count 120
`)
}
