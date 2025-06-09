package metrics

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestPrometheusHistogramSerial(t *testing.T) {
	const expected string = `prefix_bucket{le="0.005"} 1
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
`
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
	testMarshalTo(t, h, "prefix", expected)

	// make sure that if the +Inf bucket is manually specified it gets ignored and we have the same results at the end
	h2 := NewPrometheusHistogramExt("TestPrometheusHistogram2", append(PrometheusHistogramDefaultBuckets, math.Inf(+1)))

	h.Reset()

	// Write data to histogram
	for i := 0; i <= 10_100; i += 25 { // from 0 to 10'100 ms in 25ms steps
		h.Update(float64(i) * 1e-3)
		h2.Update(float64(i) * 1e-3)
	}

	// also test negative values and NaN for h, those will be ignored
	h.Update(-1)
	h.Update(math.NaN())

	// Make sure the histogram prints <prefix>_bucket on marshalTo call
	testMarshalTo(t, h, "prefix", expected)
	testMarshalTo(t, h2, "prefix", expected)
}

func TestPrometheusHistogramLinearBuckets(t *testing.T) {
	const expected string = `prefix_bucket{le="0.5"} 1
prefix_bucket{le="1.5"} 2
prefix_bucket{le="2.5"} 3
prefix_bucket{le="3.5"} 4
prefix_bucket{le="4.5"} 5
prefix_bucket{le="5.5"} 6
prefix_bucket{le="6.5"} 7
prefix_bucket{le="7.5"} 8
prefix_bucket{le="8.5"} 9
prefix_bucket{le="9.5"} 10
prefix_bucket{le="+Inf"} 11
prefix_sum 55
prefix_count 11
`
	name := "TestPrometheusHistogramLinearBuckets"
	upperBounds := LinearBuckets(0.5, 1.0, 10)
	h := NewPrometheusHistogramExt(name, upperBounds)

	// Write data to histogram
	for i := 0; i <= 10; i++ { // from 0 to 10
		h.Update(float64(i))
	}

	// Make sure the histogram prints <prefix>_bucket on marshalTo call
	testMarshalTo(t, h, "prefix", expected)

	// Make sure we panic when the count of linear buckets is < 1
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("LinearBuckets should've panicked with a count of 0.")
			}
		}()
		_ = LinearBuckets(.14, .22, 0)
	}()
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
	f := func(expOutput string) {
		t.Helper()
		var bb bytes.Buffer
		WritePrometheus(&bb, false)
		result := bb.String()
		if !strings.Contains(result, expOutput) {
			t.Fatalf("missing histogram %s in the WritePrometheus output; got\n%s", expOutput, result)
		}
	}

	h := NewPrometheusHistogram(`TestPrometheusHistogram`)
	h.Update(123)
	f(`TestPrometheusHistogram_bucket{le="+Inf"} 1`)
	f(`TestPrometheusHistogram_count 1`)
	f(`TestPrometheusHistogram_sum 123`)

	h = NewPrometheusHistogram(`TestPrometheusHistogram{tag="foo"}`)
	h.Update(123)
	f(`TestPrometheusHistogram_bucket{tag="foo",le="+Inf"} 1`)
	f(`TestPrometheusHistogram_count{tag="foo"} 1`)
	f(`TestPrometheusHistogram_sum{tag="foo"} 123`)
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
