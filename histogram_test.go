package metrics

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestHistogramUpdateNegativeValue(t *testing.T) {
	h := NewHistogram("TestHisogramUpdateNegativeValue")
	expectPanic(t, "negative value", func() {
		h.Update(-123)
	})
}

func TestGetBucketIdx(t *testing.T) {
	f := func(v float64, idxExpected uint) {
		t.Helper()
		idx := getBucketIdx(v)
		if idx != idxExpected {
			t.Fatalf("unexpected getBucketIdx(%g); got %d; want %d", v, idx, idxExpected)
		}
	}
	f(0, 0)
	f(math.Pow10(e10Min-10), 1)
	f(math.Pow10(e10Min-1), 1)
	f(1.5*math.Pow10(e10Min-1), 1)
	f(2*math.Pow10(e10Min-1), 1)
	f(3*math.Pow10(e10Min-1), 1)
	f(9*math.Pow10(e10Min-1), 1)
	f(9.999*math.Pow10(e10Min-1), 1)
	f(math.Pow10(e10Min), 1)
	f(1.00001*math.Pow10(e10Min), 2)
	f(1.5*math.Pow10(e10Min), 2)
	f(1.999999*math.Pow10(e10Min), 2)
	f(2*math.Pow10(e10Min), 2)
	f(2.0000001*math.Pow10(e10Min), 3)
	f(2.999*math.Pow10(e10Min), 3)
	f(8.999*math.Pow10(e10Min), 9)
	f(9*math.Pow10(e10Min), 9)
	f(9.01*math.Pow10(e10Min), 10)
	f(9.99999*math.Pow10(e10Min), 10)
	f(math.Pow10(e10Min+1), 10)
	f(1.9*math.Pow10(e10Min+1), 11)
	f(9.9*math.Pow10(e10Min+1), 19)
	f(math.Pow10(e10Min+2), 19)
	f(math.Pow10(e10Min+3), 28)
	f(5*math.Pow10(e10Min+3), 32)
	f(0.1, 1-9*(e10Min+1))
	f(0.11, 2-9*(e10Min+1))
	f(0.95, 1-9*e10Min)
	f(1, 1-9*e10Min)
	f(2, 2-9*e10Min)
	f(math.Pow10(e10Max), 1+9*(e10Max-e10Min))
	f(2*math.Pow10(e10Max), 2+9*(e10Max-e10Min))
	f(9.999*math.Pow10(e10Max), 10+9*(e10Max-e10Min))
	f(math.Pow10(e10Max+1), 10+9*(e10Max-e10Min))
	f(2*math.Pow10(e10Max+1), 11+9*(e10Max-e10Min))
	f(9*math.Pow10(e10Max+1), 11+9*(e10Max-e10Min))
	f(math.Pow10(e10Max+5), 11+9*(e10Max-e10Min))
	f(12.34*math.Pow10(e10Max+7), 11+9*(e10Max-e10Min))
	f(math.Inf(1), 11+9*(e10Max-e10Min))
}

func TestGetRangeEndFromBucketIdx(t *testing.T) {
	f := func(idx uint, endExpected string) {
		t.Helper()
		end := getRangeEndFromBucketIdx(idx)
		if end != endExpected {
			t.Fatalf("unexpected getRangeEndFromBucketIdx(%d); got %s; want %s", idx, end, endExpected)
		}
	}
	f(0, "0")
	f(1, fmt.Sprintf("1e%d", e10Min))
	f(2, fmt.Sprintf("2e%d", e10Min))
	f(3, fmt.Sprintf("3e%d", e10Min))
	f(9, fmt.Sprintf("9e%d", e10Min))
	f(10, fmt.Sprintf("1e%d", e10Min+1))
	f(11, fmt.Sprintf("2e%d", e10Min+1))
	f(16, fmt.Sprintf("7e%d", e10Min+1))
	f(17, fmt.Sprintf("8e%d", e10Min+1))
	f(18, fmt.Sprintf("9e%d", e10Min+1))
	f(19, fmt.Sprintf("1e%d", e10Min+2))
	f(20, fmt.Sprintf("2e%d", e10Min+2))
	f(21, fmt.Sprintf("3e%d", e10Min+2))
	f(bucketsCount-21, fmt.Sprintf("9e%d", e10Max-2))
	f(bucketsCount-20, fmt.Sprintf("1e%d", e10Max-1))
	f(bucketsCount-16, fmt.Sprintf("5e%d", e10Max-1))
	f(bucketsCount-13, fmt.Sprintf("8e%d", e10Max-1))
	f(bucketsCount-12, fmt.Sprintf("9e%d", e10Max-1))
	f(bucketsCount-11, fmt.Sprintf("1e%d", e10Max))
	f(bucketsCount-10, fmt.Sprintf("2e%d", e10Max))
	f(bucketsCount-4, fmt.Sprintf("8e%d", e10Max))
	f(bucketsCount-3, fmt.Sprintf("9e%d", e10Max))
	f(bucketsCount-2, fmt.Sprintf("1e%d", e10Max+1))
	f(bucketsCount-1, "+Inf")
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
	for i := 84; i < 324; i++ {
		h.Update(float64(i))
	}

	// Make sure the histogram prints <prefix>_xbucket on marshalTo call
	testMarshalTo(t, h, "prefix", "prefix_vmbucket{vmrange=\"8e1...9e1\"} 7\nprefix_vmbucket{vmrange=\"9e1...1e2\"} 10\nprefix_vmbucket{vmrange=\"1e2...2e2\"} 100\nprefix_vmbucket{vmrange=\"2e2...3e2\"} 100\nprefix_vmbucket{vmrange=\"3e2...4e2\"} 23\nprefix_sum 48840\nprefix_count 240\n")
	testMarshalTo(t, h, `	  m{foo="bar"}`, "\t  m_vmbucket{foo=\"bar\",vmrange=\"8e1...9e1\"} 7\n\t  m_vmbucket{foo=\"bar\",vmrange=\"9e1...1e2\"} 10\n\t  m_vmbucket{foo=\"bar\",vmrange=\"1e2...2e2\"} 100\n\t  m_vmbucket{foo=\"bar\",vmrange=\"2e2...3e2\"} 100\n\t  m_vmbucket{foo=\"bar\",vmrange=\"3e2...4e2\"} 23\n\t  m_sum{foo=\"bar\"} 48840\n\t  m_count{foo=\"bar\"} 240\n")

	// Verify supported ranges
	for i := -100; i < 100; i++ {
		h.Update(1.23 * math.Pow10(i))
	}
	h.UpdateDuration(time.Now().Add(-time.Minute))

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
		for i := 0; i < 10; i++ {
			h.Update(float64(i))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	testMarshalTo(t, h, "prefix", "prefix_vmbucket{vmrange=\"0...0\"} 5\nprefix_vmbucket{vmrange=\"9e-1...1\"} 5\nprefix_vmbucket{vmrange=\"1...2\"} 5\nprefix_vmbucket{vmrange=\"2...3\"} 5\nprefix_vmbucket{vmrange=\"3...4\"} 5\nprefix_vmbucket{vmrange=\"4...5\"} 5\nprefix_vmbucket{vmrange=\"5...6\"} 5\nprefix_vmbucket{vmrange=\"6...7\"} 5\nprefix_vmbucket{vmrange=\"7...8\"} 5\nprefix_vmbucket{vmrange=\"8...9\"} 5\nprefix_sum 225\nprefix_count 50\n")
}

func TestHistogramWithTags(t *testing.T) {
	name := `TestHistogram{tag="foo"}`
	h := NewHistogram(name)
	h.Update(123)

	var bb bytes.Buffer
	WritePrometheus(&bb, false)
	result := bb.String()
	namePrefixWithTag := `TestHistogram_vmbucket{tag="foo",vmrange="1e2...2e2"} 1` + "\n"
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
