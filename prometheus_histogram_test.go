package metrics

import (
	"bytes"
	"fmt"
	"math"
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
	for i := 0; i <= 10_100; i += 25 { // from 0 to 10'100 ms in 25ms steps
		h.Update(float64(i) * 1e-3)
	}

	// Make sure the histogram prints <prefix>_bucket on marshalTo call
	testMarshalTo(t, h, "prefix", `prefix_bucket{le="0.005"} 1
prefix_bucket{le="0.01"} 1
prefix_bucket{le="0.025"} 2
prefix_bucket{le="0.05"} 3
prefix_bucket{le="0.1"} 5
prefix_bucket{le="0.25"} 11
prefix_bucket{le="0.5"} 21
prefix_bucket{le="1"} 41
prefix_bucket{le="2.5"} 101
prefix_bucket{le="5"} 201
prefix_bucket{le="10"} 401
prefix_bucket{le="+Inf"} 405
prefix_sum 2045.25
prefix_count 405
`)

	// make sure that if the +Inf bucket is manually specified it gets ignored and we have the same resutls at the end
	h2 := NewPrometheusHistogramExt("TestPrometheusHistogram2", append(defaultUpperBounds, math.Inf(+1)))

	// Write data to histogram
	for i := 0; i <= 10_100; i += 25 { // from 0 to 10'100 ms in 25ms steps
		h2.Update(float64(i) * 1e-3)
	}

	// Make sure the histogram prints <prefix>_bucket on marshalTo call
	testMarshalTo(t, h2, "prefix", `prefix_bucket{le="0.005"} 1
prefix_bucket{le="0.01"} 1
prefix_bucket{le="0.025"} 2
prefix_bucket{le="0.05"} 3
prefix_bucket{le="0.1"} 5
prefix_bucket{le="0.25"} 11
prefix_bucket{le="0.5"} 21
prefix_bucket{le="1"} 41
prefix_bucket{le="2.5"} 101
prefix_bucket{le="5"} 201
prefix_bucket{le="10"} 401
prefix_bucket{le="+Inf"} 405
prefix_sum 2045.25
prefix_count 405
`)
}

func TestPrometheusHistogramMerge(t *testing.T) {
	name := `TestPrometheusHistogramMerge`
	h := NewPrometheusHistogram(name)
	// Write data to histogram
	for i := 0; i <= 10_100; i += 25 { // from 0 to 10'100 ms in 25ms steps
		h.Update(float64(i) * 1e-3)
	}

	b := NewPrometheusHistogram("test")
	for i := 0; i <= 10_100; i += 25 { // from 0 to 10'100 ms in 25ms steps
		h.Update(float64(i) * 1e-3)
	}

	h.Merge(b)

	// Make sure the histogram prints <prefix>_bucket on marshalTo call
	testMarshalTo(t, h, "prefix", `prefix_bucket{le="0.005"} 2
prefix_bucket{le="0.01"} 2
prefix_bucket{le="0.025"} 4
prefix_bucket{le="0.05"} 6
prefix_bucket{le="0.1"} 10
prefix_bucket{le="0.25"} 22
prefix_bucket{le="0.5"} 42
prefix_bucket{le="1"} 82
prefix_bucket{le="2.5"} 202
prefix_bucket{le="5"} 402
prefix_bucket{le="10"} 802
prefix_bucket{le="+Inf"} 810
prefix_sum 4090.5
prefix_count 810
`)
}

// inspired from https://github.com/prometheus/client_golang/blob/main/prometheus/histogram_test.go
func TestPrometheusHistogramNonMonotonicBuckets(t *testing.T) {
	testCases := map[string][]float64{
		"0 bucket is invalid":     {},
		"not strictly monotonic":  {1, 2, 2, 3},
		"not monotonic at all":    {1, 2, 4, 3, 5},
		"have +Inf in the middle": {1, 2, math.Inf(+1), 3},
	}
	for name, buckets := range testCases {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Buckets %v are %s but NewPrometheusHistogram did not panic.", buckets, name)
				}
			}()
			_ = NewPrometheusHistogramExt("test", buckets)
		}()
	}
}

func TestPrometheusHistogramWithTags(t *testing.T) {
	name := `TestPrometheusHistogram{tag="foo"}`
	h := NewPrometheusHistogram(name)
	h.Update(123)

	var bb bytes.Buffer
	WritePrometheus(&bb, false)
	result := bb.String()
	namePrefixWithTag := `TestPrometheusHistogram_bucket{tag="foo",le="+Inf"} 1` + "\n"
	if !strings.Contains(result, namePrefixWithTag) {
		t.Fatalf("missing histogram %s in the WritePrometheus output; got\n%s", namePrefixWithTag, result)
	}
}

func TestPrometheusHistogramWithEmptyTags(t *testing.T) {
	name := `TestPrometheusHistogram{}`
	h := NewPrometheusHistogram(name)
	h.Update(123)

	var bb bytes.Buffer
	WritePrometheus(&bb, false)
	result := bb.String()
	namePrefixWithTag := `TestPrometheusHistogram_bucket{le="+Inf"} 1` + "\n"
	if !strings.Contains(result, namePrefixWithTag) {
		t.Fatalf("missing histogram %s in the WritePrometheus output; got\n%s", namePrefixWithTag, result)
	}
}

func TestGetOrCreatePrometheusHistogramSerial(t *testing.T) {
	name := "GetOrCreatePrometheusHistogramSerial"
	if err := testGetOrCreatePrometheusHistogram(name); err != nil {
		t.Fatal(err)
	}
}

func TestGetOrCreatePrometheusHistogramConcurrent(t *testing.T) {
	name := "GetOrCreatePrometheusHistogramConcurrent"
	err := testConcurrent(func() error {
		return testGetOrCreatePrometheusHistogram(name)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func testGetOrCreatePrometheusHistogram(name string) error {
	h1 := GetOrCreatePrometheusHistogram(name)
	for i := 0; i < 10; i++ {
		h2 := GetOrCreatePrometheusHistogram(name)
		if h1 != h2 {
			return fmt.Errorf("unexpected histogram returned; got %p; want %p", h2, h1)
		}
	}
	return nil
}
