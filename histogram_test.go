package metrics

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGetVMRange(t *testing.T) {
	f := func(bucketIdx int, vmrangeExpected string) {
		t.Helper()
		vmrange := getVMRange(bucketIdx)
		if vmrange != vmrangeExpected {
			t.Fatalf("unexpected vmrange for bucketIdx=%d; got %s; want %s", bucketIdx, vmrange, vmrangeExpected)
		}
	}
	f(0, "1.000e-09...1.136e-09")
	f(1, "1.136e-09...1.292e-09")
	f(bucketsPerDecimal-1, "8.799e-09...1.000e-08")
	f(bucketsPerDecimal, "1.000e-08...1.136e-08")
	f(bucketsPerDecimal*(-e10Min)-1, "8.799e-01...1.000e+00")
	f(bucketsPerDecimal*(-e10Min), "1.000e+00...1.136e+00")
	f(bucketsPerDecimal*(e10Max-e10Min)-1, "8.799e+17...1.000e+18")
}

func TestHistogramSerial(t *testing.T) {
	name := `TestHistogramSerial`
	h := NewHistogram(name)

	// Verify that the histogram is invisible in the output of WritePrometheus when it has no data.
	var bb bytes.Buffer
	WritePrometheus(&bb, false)
	result := bb.String()
	if strings.Contains(result, name) {
		t.Fatalf("histogram %s shouldn't be visible in the WritePrometheus output; got\n%s", name, result)
	}

	// Write data to histogram
	for i := 98; i < 218; i++ {
		h.Update(float64(i))
	}

	// Make sure the histogram prints <prefix>_bucket on marshalTo call
	testMarshalTo(t, h, "prefix", `prefix_bucket{vmrange="8.799e+01...1.000e+02"} 3
prefix_bucket{vmrange="1.000e+02...1.136e+02"} 13
prefix_bucket{vmrange="1.136e+02...1.292e+02"} 16
prefix_bucket{vmrange="1.292e+02...1.468e+02"} 17
prefix_bucket{vmrange="1.468e+02...1.668e+02"} 20
prefix_bucket{vmrange="1.668e+02...1.896e+02"} 23
prefix_bucket{vmrange="1.896e+02...2.154e+02"} 26
prefix_bucket{vmrange="2.154e+02...2.448e+02"} 2
prefix_sum 18900
prefix_count 120
`)
	testMarshalTo(t, h, `	  m{foo="bar"}`, `	  m_bucket{foo="bar",vmrange="8.799e+01...1.000e+02"} 3
	  m_bucket{foo="bar",vmrange="1.000e+02...1.136e+02"} 13
	  m_bucket{foo="bar",vmrange="1.136e+02...1.292e+02"} 16
	  m_bucket{foo="bar",vmrange="1.292e+02...1.468e+02"} 17
	  m_bucket{foo="bar",vmrange="1.468e+02...1.668e+02"} 20
	  m_bucket{foo="bar",vmrange="1.668e+02...1.896e+02"} 23
	  m_bucket{foo="bar",vmrange="1.896e+02...2.154e+02"} 26
	  m_bucket{foo="bar",vmrange="2.154e+02...2.448e+02"} 2
	  m_sum{foo="bar"} 18900
	  m_count{foo="bar"} 120
`)

	// Verify Reset
	h.Reset()
	bb.Reset()
	WritePrometheus(&bb, false)
	result = bb.String()
	if strings.Contains(result, name) {
		t.Fatalf("unexpected histogram %s in the WritePrometheus output; got\n%s", name, result)
	}

	// Verify supported ranges
	for e10 := -100; e10 < 100; e10++ {
		for offset := 0; offset < bucketsPerDecimal; offset++ {
			m := 1 + math.Pow(bucketMultiplier, float64(offset))
			f1 := m * math.Pow10(e10)
			h.Update(f1)
			f2 := (m + 0.5*bucketMultiplier) * math.Pow10(e10)
			h.Update(f2)
			f3 := (m + 2*bucketMultiplier) * math.Pow10(e10)
			h.Update(f3)
		}
	}
	h.UpdateDuration(time.Now().Add(-time.Minute))

	// Verify edge cases
	h.Update(0)
	h.Update(math.Inf(1))
	h.Update(math.Inf(-1))
	h.Update(math.NaN())
	h.Update(-123)
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1096
	h.Update(math.Float64frombits(0x3e112e0be826d695))

	// Make sure the histogram becomes visible in the output of WritePrometheus,
	// since now it contains values.
	bb.Reset()
	WritePrometheus(&bb, false)
	result = bb.String()
	if !strings.Contains(result, name) {
		t.Fatalf("missing histogram %s in the WritePrometheus output; got\n%s", name, result)
	}
}

func TestHistogramConcurrent(t *testing.T) {
	name := "HistogramConcurrent"
	h := NewHistogram(name)
	err := testConcurrent(func() error {
		for f := 0.6; f < 1.4; f += 0.1 {
			h.Update(f)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	testMarshalTo(t, h, "prefix", `prefix_bucket{vmrange="5.995e-01...6.813e-01"} 5
prefix_bucket{vmrange="6.813e-01...7.743e-01"} 5
prefix_bucket{vmrange="7.743e-01...8.799e-01"} 5
prefix_bucket{vmrange="8.799e-01...1.000e+00"} 10
prefix_bucket{vmrange="1.000e+00...1.136e+00"} 5
prefix_bucket{vmrange="1.136e+00...1.292e+00"} 5
prefix_bucket{vmrange="1.292e+00...1.468e+00"} 5
prefix_sum 38
prefix_count 40
`)

	var labels []string
	var counts []uint64
	h.VisitNonZeroBuckets(func(label string, count uint64) {
		labels = append(labels, label)
		counts = append(counts, count)
	})
	labelsExpected := []string{
		"5.995e-01...6.813e-01",
		"6.813e-01...7.743e-01",
		"7.743e-01...8.799e-01",
		"8.799e-01...1.000e+00",
		"1.000e+00...1.136e+00",
		"1.136e+00...1.292e+00",
		"1.292e+00...1.468e+00",
	}
	if !reflect.DeepEqual(labels, labelsExpected) {
		t.Fatalf("unexpected labels; got %v; want %v", labels, labelsExpected)
	}
	countsExpected := []uint64{5, 5, 5, 10, 5, 5, 5}
	if !reflect.DeepEqual(counts, countsExpected) {
		t.Fatalf("unexpected counts; got %v; want %v", counts, countsExpected)
	}
}

func TestHistogramWithTags(t *testing.T) {
	name := `TestHistogram{tag="foo"}`
	h := NewHistogram(name)
	h.Update(123)

	var bb bytes.Buffer
	WritePrometheus(&bb, false)
	result := bb.String()
	namePrefixWithTag := `TestHistogram_bucket{tag="foo",vmrange="1.136e+02...1.292e+02"} 1` + "\n"
	if !strings.Contains(result, namePrefixWithTag) {
		t.Fatalf("missing histogram %s in the WritePrometheus output; got\n%s", namePrefixWithTag, result)
	}
}

func TestGetOrCreateHistogramSerial(t *testing.T) {
	name := "GetOrCreateHistogramSerial"
	if err := testGetOrCreateHistogram(name); err != nil {
		t.Fatal(err)
	}
}

func TestGetOrCreateHistogramConcurrent(t *testing.T) {
	name := "GetOrCreateHistogramConcurrent"
	err := testConcurrent(func() error {
		return testGetOrCreateHistogram(name)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func testGetOrCreateHistogram(name string) error {
	h1 := GetOrCreateHistogram(name)
	for i := 0; i < 10; i++ {
		h2 := GetOrCreateHistogram(name)
		if h1 != h2 {
			return fmt.Errorf("unexpected histogram returned; got %p; want %p", h2, h1)
		}
	}
	return nil
}
