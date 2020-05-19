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

func TestGetBucketIdxAndOffset(t *testing.T) {
	f := func(v float64, bucketIdxExpected int, offsetExpected uint) {
		t.Helper()
		bucketIdx, offset := getBucketIdxAndOffset(v)
		if bucketIdx != bucketIdxExpected {
			t.Fatalf("unexpected bucketIdx for %g; got %d; want %d", v, bucketIdx, bucketIdxExpected)
		}
		if offset != offsetExpected {
			t.Fatalf("unexpected offset for %g; got %d; want %d", v, offset, offsetExpected)
		}
	}
	const step = 1.0 / decimalMultiplier
	const prec = 2 * decimalPrecision
	f(0, -1, 0)
	f(math.Pow10(e10Min-10), -1, 1)
	f(math.Pow10(e10Min-1), -1, 1)
	f(3*math.Pow10(e10Min-1), -1, 1)
	f(9*math.Pow10(e10Min-1), -1, 1)
	f(9.999*math.Pow10(e10Min-1), -1, 1)
	f(math.Pow10(e10Min), -1, 1)
	f((1+prec)*math.Pow10(e10Min), 0, 0)
	f((1+step)*math.Pow10(e10Min), 0, 0)
	f((1+step+prec)*math.Pow10(e10Min), 0, 1)
	f((1+2*step+prec)*math.Pow10(e10Min), 0, 2)
	f((1+3*step+prec)*math.Pow10(e10Min), 0, 3)
	f(math.Pow10(e10Min+1), 0, bucketSize-1)
	f((1+prec)*math.Pow10(e10Min+1), 1, 0)
	f((1+step)*math.Pow10(e10Min+1), 1, 0)
	f((1+step+prec)*math.Pow10(e10Min+1), 1, 1)
	f(0.1, -e10Min-2, bucketSize-1)
	f((1+prec)*0.1, -e10Min-1, 0)
	f((1+step)*0.1, -e10Min-1, 0)
	f((1+step+prec)*0.1, -e10Min-1, 1)
	f((1+(bucketSize-1)*step)*0.1, -e10Min-1, bucketSize-2)
	f((1+(bucketSize-1)*step+prec)*0.1, -e10Min-1, bucketSize-1)
	f(math.Pow10(e10Max-2), bucketsCount-3, bucketSize-1)
	f((1+prec)*math.Pow10(e10Max-2), bucketsCount-2, 0)
	f(math.Pow10(e10Max-1), bucketsCount-2, bucketSize-1)
	f((1+prec)*math.Pow10(e10Max-1), bucketsCount-1, 0)
	f((1+(bucketSize-1)*step)*math.Pow10(e10Max-1), bucketsCount-1, bucketSize-2)
	f((1+(bucketSize-1)*step+prec)*math.Pow10(e10Max-1), bucketsCount-1, bucketSize-1)
	f(math.Pow10(e10Max), bucketsCount-1, bucketSize-1)
	f((1+prec)*math.Pow10(e10Max), -1, 2)
	f((1+3*step+prec)*math.Pow10(e10Max), -1, 2)
	f(math.Inf(1), -1, 2)

	f(999, 11, 17)
	f(1000, 11, 17)
	f(1001, 12, 0)
	f(1002, 12, 0)
	f(1003, 12, 0)
}

func TestGetVMRange(t *testing.T) {
	f := func(bucketIdx int, offset uint, vmrangeExpected string) {
		t.Helper()
		vmrange := getVMRange(bucketIdx, offset)
		if vmrange != vmrangeExpected {
			t.Fatalf("unexpected vmrange for bucketIdx=%d, offset=%d; got %s; want %s", bucketIdx, offset, vmrange, vmrangeExpected)
		}
	}
	const step = 1.0 / decimalMultiplier
	f(-1, 0, "0...0")
	f(-1, 1, fmt.Sprintf("0...1.0e%d", e10Min))
	f(-1, 2, fmt.Sprintf("1.0e%d...+Inf", e10Max))
	f(0, 0, fmt.Sprintf("1.0e%d...%.1fe%d", e10Min, 1+step, e10Min))
	f(0, 1, fmt.Sprintf("%.1fe%d...%.1fe%d", 1+step, e10Min, 1+2*step, e10Min))
	f(0, bucketSize-2, fmt.Sprintf("%.1fe%d...%.1fe%d", 1+(bucketSize-2)*step, e10Min, 1+(bucketSize-1)*step, e10Min))
	f(0, bucketSize-1, fmt.Sprintf("%.1fe%d...%.1fe%d", 1+(bucketSize-1)*step, e10Min, 1.0, e10Min+1))
	f(-e10Min, 0, fmt.Sprintf("%.1fe%d...%.1fe%d", 1.0, 0, 1+step, 0))
	f(-e10Min, 1, fmt.Sprintf("%.1fe%d...%.1fe%d", 1+step, 0, 1+2*step, 0))
	f(-e10Min, bucketSize-2, fmt.Sprintf("%.1fe%d...%.1fe%d", 1+(bucketSize-2)*step, 0, 1+(bucketSize-1)*step, 0))
	f(-e10Min, bucketSize-1, fmt.Sprintf("%.1fe%d...%.1fe%d", 1+(bucketSize-1)*step, 0, 1.0, 1))
	f(bucketsCount-1, bucketSize-2, fmt.Sprintf("%.1fe%d...%.1fe%d", 1+(bucketSize-2)*step, e10Max-1, 1+(bucketSize-1)*step, e10Max-1))
	f(bucketsCount-1, bucketSize-1, fmt.Sprintf("%.1fe%d...%.1fe%d", 1+(bucketSize-1)*step, e10Max-1, 1.0, e10Max))
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

	// Make sure the histogram prints <prefix>_xbucket on marshalTo call
	testMarshalTo(t, h, "prefix", "prefix_bucket{vmrange=\"9.5e1...1.0e2\"} 3\nprefix_bucket{vmrange=\"1.0e2...1.5e2\"} 50\nprefix_bucket{vmrange=\"1.5e2...2.0e2\"} 50\nprefix_bucket{vmrange=\"2.0e2...2.5e2\"} 17\nprefix_sum 18900\nprefix_count 120\n")
	testMarshalTo(t, h, `	  m{foo="bar"}`, "\t  m_bucket{foo=\"bar\",vmrange=\"9.5e1...1.0e2\"} 3\n\t  m_bucket{foo=\"bar\",vmrange=\"1.0e2...1.5e2\"} 50\n\t  m_bucket{foo=\"bar\",vmrange=\"1.5e2...2.0e2\"} 50\n\t  m_bucket{foo=\"bar\",vmrange=\"2.0e2...2.5e2\"} 17\n\t  m_sum{foo=\"bar\"} 18900\n\t  m_count{foo=\"bar\"} 120\n")

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
		for offset := 0; offset < bucketSize; offset++ {
			m := 1 + float64(offset+1)/decimalMultiplier
			f1 := m * math.Pow10(e10)
			h.Update(f1)
			f2 := (m + 0.5/decimalMultiplier) * math.Pow10(e10)
			h.Update(f2)
			f3 := (m + 2*decimalPrecision) * math.Pow10(e10)
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
	testMarshalTo(t, h, "prefix", "prefix_bucket{vmrange=\"5.5e-1...6.0e-1\"} 5\nprefix_bucket{vmrange=\"6.5e-1...7.0e-1\"} 5\nprefix_bucket{vmrange=\"7.5e-1...8.0e-1\"} 5\nprefix_bucket{vmrange=\"8.5e-1...9.0e-1\"} 5\nprefix_bucket{vmrange=\"9.5e-1...1.0e0\"} 5\nprefix_bucket{vmrange=\"1.0e0...1.5e0\"} 15\nprefix_sum 38\nprefix_count 40\n")

	var labels []string
	var counts []uint64
	h.VisitNonZeroBuckets(func(label string, count uint64) {
		labels = append(labels, label)
		counts = append(counts, count)
	})
	labelsExpected := []string{"5.5e-1...6.0e-1", "6.5e-1...7.0e-1", "7.5e-1...8.0e-1", "8.5e-1...9.0e-1", "9.5e-1...1.0e0", "1.0e0...1.5e0"}
	if !reflect.DeepEqual(labels, labelsExpected) {
		t.Fatalf("unexpected labels; got %v; want %v", labels, labelsExpected)
	}
	countsExpected := []uint64{5, 5, 5, 5, 5, 15}
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
	namePrefixWithTag := `TestHistogram_bucket{tag="foo",vmrange="1.0e2...1.5e2"} 1` + "\n"
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
