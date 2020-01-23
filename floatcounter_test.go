package metrics

import (
	"fmt"
	"testing"
)

func TestFloatCounterSerial(t *testing.T) {
	name := "FloatCounterSerial"
	c := NewFloatCounter(name)
	c.Add(0.1)
	if n := c.Get(); n != 0.1 {
		t.Fatalf("unexpected counter value; got %f; want 0.1", n)
	}
	c.Set(123.00001)
	if n := c.Get(); n != 123.00001 {
		t.Fatalf("unexpected counter value; got %f; want 123.00001", n)
	}
	c.Sub(0.00001)
	if n := c.Get(); n != 123 {
		t.Fatalf("unexpected counter value; got %f; want 123", n)
	}
	c.Add(2.002)
	if n := c.Get(); n != 125.002 {
		t.Fatalf("unexpected counter value; got %f; want 125.002", n)
	}

	// Verify MarshalTo
	testMarshalTo(t, c, "foobar", "foobar 125.002\n")
}

func TestFloatCounterConcurrent(t *testing.T) {
	name := "FloatCounterConcurrent"
	c := NewFloatCounter(name)
	err := testConcurrent(func() error {
		nPrev := c.Get()
		for i := 0; i < 10; i++ {
			c.Add(1.001)
			if n := c.Get(); n <= nPrev {
				return fmt.Errorf("counter value must be greater than %f; got %f", nPrev, n)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetOrCreateFloatCounterSerial(t *testing.T) {
	name := "GetOrCreateFloatCounterSerial"
	if err := testGetOrCreateCounter(name); err != nil {
		t.Fatal(err)
	}
}

func TestGetOrCreateFloatCounterConcurrent(t *testing.T) {
	name := "GetOrCreateFloatCounterConcurrent"
	err := testConcurrent(func() error {
		return testGetOrCreateFloatCounter(name)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func testGetOrCreateFloatCounter(name string) error {
	c1 := GetOrCreateFloatCounter(name)
	for i := 0; i < 10; i++ {
		c2 := GetOrCreateFloatCounter(name)
		if c1 != c2 {
			return fmt.Errorf("unexpected counter returned; got %p; want %p", c2, c1)
		}
	}
	return nil
}
