package metrics

import (
	"fmt"
	"io"
	"testing"
)

func TestCounterSerial(t *testing.T) {
	name := "CounterSerial"
	c := NewCounter(name)
	c.Inc()
	if n := c.Get(); n != 1 {
		t.Fatalf("unexpected counter value; got %d; want 1", n)
	}
	c.Set(123)
	if n := c.Get(); n != 123 {
		t.Fatalf("unexpected counter value; got %d; want 123", n)
	}
	c.Dec()
	if n := c.Get(); n != 122 {
		t.Fatalf("unexpected counter value; got %d; want 122", n)
	}
	c.Add(3)
	if n := c.Get(); n != 125 {
		t.Fatalf("unexpected counter value; got %d; want 125", n)
	}

	// Verify MarshalTo
	testMarshalTo(t, c, "foobar", "foobar 125\n")
}

func TestCounterConcurrent(t *testing.T) {
	name := "CounterConcurrent"
	c := NewCounter(name)
	err := testConcurrent(func() error {
		nPrev := c.Get()
		for range 10 {
			c.Inc()
			if n := c.Get(); n <= nPrev {
				return fmt.Errorf("counter value must be greater than %d; got %d", nPrev, n)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetOrCreateCounterSerial(t *testing.T) {
	name := "GetOrCreateCounterSerial"
	if err := testGetOrCreateCounter(name); err != nil {
		t.Fatal(err)
	}
}

func TestGetOrCreateCounterConcurrent(t *testing.T) {
	name := "GetOrCreateCounterConcurrent"
	err := testConcurrent(func() error {
		return testGetOrCreateCounter(name)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func testGetOrCreateCounter(name string) error {
	c1 := GetOrCreateCounter(name)
	for range 10 {
		c2 := GetOrCreateCounter(name)
		if c1 != c2 {
			return fmt.Errorf("unexpected counter returned; got %p; want %p", c2, c1)
		}
	}
	return nil
}

func BenchmarkCounter_WritePrometheus(b *testing.B) {
	s := NewSet()
	c := s.NewCounter("benchmark_counter_total")
	c.Set(123456)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.WritePrometheus(io.Discard)
	}
}
