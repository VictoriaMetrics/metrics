package metrics

import (
	"fmt"
	"sync"
	"testing"
)

func TestGaugeError(t *testing.T) {
	expectPanic(t, "NewGauge_Set_non-nil-callback", func() {
		g := NewGauge("NewGauge_non_nil_callback", func() float64 { return 123 })
		g.Set(12.35)
	})
	expectPanic(t, "GetOrCreateGauge_Set_non-nil-callback", func() {
		g := GetOrCreateGauge("GetOrCreateGauge_nil_callback", func() float64 { return 123 })
		g.Set(42)
	})
}

func TestGaugeSet(t *testing.T) {
	s := NewSet()
	g := s.NewGauge("foo", nil)
	if n := g.Get(); n != 0 {
		t.Fatalf("unexpected gauge value: %g; expecting 0", n)
	}
	g.Set(1.234)
	if n := g.Get(); n != 1.234 {
		t.Fatalf("unexpected gauge value %g; expecting 1.234", n)
	}
}

func TestGaugeAdd(t *testing.T) {
	s := NewSet()
	g := s.NewGauge("foo", nil)
	if n := g.Get(); n != 0 {
		t.Fatalf("unexpected gauge value: %g; expecting 0", n)
	}
	g.Add(1.234)
	if n := g.Get(); n != 1.234 {
		t.Fatalf("unexpected gauge value %g; expecting 1.234", n)
	}
	g.Add(2.345)
	if n := g.Get(); n != 3.579 {
		t.Fatalf("unexpected gauge value %g; expecting 3.579", n)
	}
	g.Add(-1.234)
	if n := g.Get(); n != 2.345 {
		t.Fatalf("unexpected gauge value %g; expecting 2.345", n)
	}
}

func TestGaugeSerial(t *testing.T) {
	name := "GaugeSerial"
	n := 1.23
	var nLock sync.Mutex
	g := NewGauge(name, func() float64 {
		nLock.Lock()
		defer nLock.Unlock()
		n++
		return n
	})
	for i := 0; i < 10; i++ {
		if nn := g.Get(); nn != n {
			t.Fatalf("unexpected gauge value; got %v; want %v", nn, n)
		}
	}

	// Verify marshalTo
	testMarshalTo(t, g, "foobar", "foobar 12.23\n")

	// Verify big numbers marshaling
	n = 1234567899
	testMarshalTo(t, g, "prefix", "prefix 1234567900\n")
}

func TestGaugeConcurrent(t *testing.T) {
	name := "GaugeConcurrent"
	var n int
	var nLock sync.Mutex
	g := NewGauge(name, func() float64 {
		nLock.Lock()
		defer nLock.Unlock()
		n++
		return float64(n)
	})
	err := testConcurrent(func() error {
		nPrev := g.Get()
		for i := 0; i < 10; i++ {
			if n := g.Get(); n <= nPrev {
				return fmt.Errorf("gauge value must be greater than %v; got %v", nPrev, n)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
