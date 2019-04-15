package metrics

import (
	"fmt"
	"sync"
	"testing"
)

func TestGaugeError(t *testing.T) {
	expectPanic(t, "NewGauge_nil_callback", func() {
		NewGauge("NewGauge_nil_callback", nil)
	})
	expectPanic(t, "GetOrCreateGauge_nil_callback", func() {
		GetOrCreateGauge("GetOrCreateGauge_nil_callback", nil)
	})
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
