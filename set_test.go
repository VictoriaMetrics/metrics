package metrics

import (
	"fmt"
	"testing"
)

func TestNewSet(t *testing.T) {
	var ss []*Set
	for i := 0; i < 10; i++ {
		s := NewSet()
		ss = append(ss, s)
	}
	for i := 0; i < 10; i++ {
		s := ss[i]
		for j := 0; j < 10; j++ {
			c := s.NewCounter(fmt.Sprintf("counter_%d", j))
			c.Inc()
			if n := c.Get(); n != 1 {
				t.Fatalf("unexpected counter value; got %d; want %d", n, 1)
			}
			g := s.NewGauge(fmt.Sprintf("gauge_%d", j), func() float64 { return 123 })
			if v := g.Get(); v != 123 {
				t.Fatalf("unexpected gauge value; got %v; want %v", v, 123)
			}
			sm := s.NewSummary(fmt.Sprintf("summary_%d", j))
			if sm == nil {
				t.Fatalf("NewSummary returned nil")
			}
		}
	}
}
