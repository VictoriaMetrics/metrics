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
	expectPanic(t, "GetOrCreateGauge_Add_non-nil-callback", func() {
		g := GetOrCreateGauge("GetOrCreateGauge_nil_callback", func() float64 { return 123 })
		g.Add(42)
	})
	expectPanic(t, "GetOrCreateGauge_Inc_non-nil-callback", func() {
		g := GetOrCreateGauge("GetOrCreateGauge_nil_callback", func() float64 { return 123 })
		g.Inc()
	})
	expectPanic(t, "GetOrCreateGauge_Dec_non-nil-callback", func() {
		g := GetOrCreateGauge("GetOrCreateGauge_nil_callback", func() float64 { return 123 })
		g.Dec()
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

func TestGaugeIncDec(t *testing.T) {
	s := NewSet()
	g := s.NewGauge("foo", nil)
	if n := g.Get(); n != 0 {
		t.Fatalf("unexpected gauge value: %g; expecting 0", n)
	}
	for i := 1; i <= 100; i++ {
		g.Inc()
		if n := g.Get(); n != float64(i) {
			t.Fatalf("unexpected gauge value %g; expecting %d", n, i)
		}
	}
	for i := 99; i >= 0; i-- {
		g.Dec()
		if n := g.Get(); n != float64(i) {
			t.Fatalf("unexpected gauge value %g; expecting %d", n, i)
		}
	}
}

func TestGaugeIncDecConcurrenc(t *testing.T) {
	s := NewSet()
	g := s.NewGauge("foo", nil)

	workers := 5
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < 100; i++ {
				g.Inc()
				g.Dec()
			}
			wg.Done()
		}()
	}
	wg.Wait()

	if n := g.Get(); n != 0 {
		t.Fatalf("unexpected gauge value %g; want 0", n)
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
