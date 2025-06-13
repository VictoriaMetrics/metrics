package metrics

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestPrometheusHistogramEmpty(t *testing.T) {
	const expected string = `empty_bucket{le="1"} 0
empty_bucket{le="2"} 0
empty_bucket{le="4"} 0
empty_bucket{le="+Inf"} 0
empty_sum 0
empty_count 0
`
	// histogram without registered observations should be still rendered
	h := NewPrometheusHistogramExt("empty", []float64{1, 2, 4})
	testMarshalTo(t, h, "empty", expected)
}

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

	// Update histogram
	for i := 0; i <= 10_100; i += 25 { // from 0 to 10'100 ms in 25ms steps
		h.Update(float64(i) * 1e-3)
	}

	// Make sure the histogram prints <prefix>_bucket on marshalTo call
	testMarshalTo(t, h, "prefix", expected)

	// make sure that if the +Inf bucket is manually specified it gets ignored and we have the same results at the end
	h2 := NewPrometheusHistogramExt("TestPrometheusHistogram2", append(PrometheusHistogramDefaultBuckets, math.Inf(+1)))

	// reset h to validate that it works
	h.Reset()

	// Update both histograms with valid values
	for i := 0; i <= 10_100; i += 25 { // from 0 to 10'100 ms in 25ms steps
		h.Update(float64(i) * 1e-3)
		h2.Update(float64(i) * 1e-3)
	}

	// update with negative and NaN values, those must be ignored
	h.Update(-1)
	h2.Update(math.NaN())

	// Make sure the histogram prints <prefix>_bucket on marshalTo call
	testMarshalTo(t, h, "prefix", expected)
	testMarshalTo(t, h2, "prefix", expected)
}

func TestPrometheusHistogramLinearBuckets(t *testing.T) {
	f := func(start, width float64, count int, exp []float64) {
		t.Helper()
		got := LinearBuckets(start, width, count)
		if !reflect.DeepEqual(got, exp) {
			t.Fatalf("expected to get: \n%v\ngot:\n%v", exp, got)
		}
	}
	f(0.5, 1.0, 4, []float64{0.5, 1.5, 2.5, 3.5})
	f(1, 4, 4, []float64{1, 5, 9, 13})
	f(-5, 5, 3, []float64{-5, 0, 5})

	fRecover := func(start, width float64, count int) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("test expected to fail with panic")
			}
		}()
		f(start, width, count, nil)
	}
	// make sure we panic for bad input
	fRecover(0.5, 1.0, 0)
	fRecover(0.5, -1, 2)
}

func TestPrometheusHistogramExponentialBuckets(t *testing.T) {
	f := func(start, factor float64, count int, exp []float64) {
		t.Helper()
		got := ExponentialBuckets(start, factor, count)
		if !reflect.DeepEqual(got, exp) {
			t.Fatalf("expected to get: \n%v\ngot:\n%v", exp, got)
		}
	}
	f(1, 2, 4, []float64{1, 2, 4, 8})
	f(0.5, 4, 4, []float64{0.5, 2, 8, 32})

	fRecover := func(start, factor float64, count int) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("test expected to fail with panic")
			}
		}()
		f(start, factor, count, nil)
	}
	// make sure we panic for bad input
	fRecover(1, 2, 0)
	fRecover(1, 1, 1)
	fRecover(0, 2, 1)
}

// inspired from https://github.com/prometheus/client_golang/blob/main/prometheus/histogram_test.go
func TestPrometheusHistogramNonMonotonicBuckets(t *testing.T) {
	i := 0
	f := func(name string, upperBounds []float64) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Buckets %v are %s but NewPrometheusHistogram did not panic.", upperBounds, name)
			}
		}()
		_ = NewPrometheusHistogramExt(fmt.Sprintf("test_%d", i), upperBounds)
	}

	f("0 bucket is invalid", []float64{})
	f("not strictly monotonic", []float64{1, 2, 2, 3})
	f("not monotonic at all", []float64{1, 2, 4, 3, 5})
	f("have +Inf in the middle", []float64{1, 2, math.Inf(+1), 3})
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
