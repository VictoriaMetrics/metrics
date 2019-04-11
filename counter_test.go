package metrics

import (
	"fmt"
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
		for i := 0; i < 10; i++ {
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
	for i := 0; i < 10; i++ {
		c2 := GetOrCreateCounter(name)
		if c1 != c2 {
			return fmt.Errorf("unexpected counter returned; got %p; want %p", c2, c1)
		}
	}
	return nil
}
