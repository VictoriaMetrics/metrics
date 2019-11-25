package metrics

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestInvalidName(t *testing.T) {
	f := func(name string) {
		t.Helper()
		expectPanic(t, fmt.Sprintf("NewCounter(%q)", name), func() { NewCounter(name) })
		expectPanic(t, fmt.Sprintf("NewGauge(%q)", name), func() { NewGauge(name, func() float64 { return 0 }) })
		expectPanic(t, fmt.Sprintf("NewSummary(%q)", name), func() { NewSummary(name) })
		expectPanic(t, fmt.Sprintf("GetOrCreateCounter(%q)", name), func() { GetOrCreateCounter(name) })
		expectPanic(t, fmt.Sprintf("GetOrCreateGauge(%q)", name), func() { GetOrCreateGauge(name, func() float64 { return 0 }) })
		expectPanic(t, fmt.Sprintf("GetOrCreateSummary(%q)", name), func() { GetOrCreateSummary(name) })
		expectPanic(t, fmt.Sprintf("GetOrCreateHistogram(%q)", name), func() { GetOrCreateHistogram(name) })
	}
	f("")
	f("foo{")
	f("foo}")
	f("foo{bar")
	f("foo{bar=")
	f(`foo{bar="`)
	f(`foo{bar="baz`)
	f(`foo{bar="baz"`)
	f(`foo{bar="baz",`)
	f(`foo{bar="baz",}`)
}

func TestDoubleRegister(t *testing.T) {
	t.Run("NewCounter", func(t *testing.T) {
		name := "NewCounterDoubleRegister"
		NewCounter(name)
		expectPanic(t, name, func() { NewCounter(name) })
	})
	t.Run("NewGauge", func(t *testing.T) {
		name := "NewGaugeDoubleRegister"
		NewGauge(name, func() float64 { return 0 })
		expectPanic(t, name, func() { NewGauge(name, func() float64 { return 0 }) })
	})
	t.Run("NewSummary", func(t *testing.T) {
		name := "NewSummaryDoubleRegister"
		NewSummary(name)
		expectPanic(t, name, func() { NewSummary(name) })
	})
	t.Run("NewHistogram", func(t *testing.T) {
		name := "NewHistogramDoubleRegister"
		NewHistogram(name)
		expectPanic(t, name, func() { NewSummary(name) })
	})
}

func TestGetOrCreateNotCounter(t *testing.T) {
	name := "GetOrCreateNotCounter"
	NewSummary(name)
	expectPanic(t, name, func() { GetOrCreateCounter(name) })
}

func TestGetOrCreateNotGauge(t *testing.T) {
	name := "GetOrCreateNotGauge"
	NewCounter(name)
	expectPanic(t, name, func() { GetOrCreateGauge(name, func() float64 { return 0 }) })
}

func TestGetOrCreateNotSummary(t *testing.T) {
	name := "GetOrCreateNotSummary"
	NewCounter(name)
	expectPanic(t, name, func() { GetOrCreateSummary(name) })
}

func TestGetOrCreateNotHistogram(t *testing.T) {
	name := "GetOrCreateNotHistogram"
	NewCounter(name)
	expectPanic(t, name, func() { GetOrCreateHistogram(name) })
}

func TestWritePrometheusSerial(t *testing.T) {
	if err := testWritePrometheus(); err != nil {
		t.Fatal(err)
	}
}

func TestWritePrometheusConcurrent(t *testing.T) {
	if err := testConcurrent(testWritePrometheus); err != nil {
		t.Fatal(err)
	}
}

func testWritePrometheus() error {
	var bb bytes.Buffer
	WritePrometheus(&bb, false)
	resultWithoutProcessMetrics := bb.String()
	bb.Reset()
	WritePrometheus(&bb, true)
	resultWithProcessMetrics := bb.String()
	if len(resultWithProcessMetrics) <= len(resultWithoutProcessMetrics) {
		return fmt.Errorf("result with process metrics must contain more data than the result without process metrics; got\n%q\nvs\n%q",
			resultWithProcessMetrics, resultWithoutProcessMetrics)
	}
	return nil
}

func expectPanic(t *testing.T, context string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		if r := recover(); r == nil {
			t.Fatalf("expecting panic in %s", context)
		}
	}()
	f()
}

func testConcurrent(f func() error) error {
	const concurrency = 5
	resultsCh := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			resultsCh <- f()
		}()
	}
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-resultsCh:
			if err != nil {
				return fmt.Errorf("unexpected error: %s", err)
			}
		case <-time.After(time.Second * 5):
			return fmt.Errorf("timeout")
		}
	}
	return nil
}

func testMarshalTo(t *testing.T, m metric, prefix, resultExpected string) {
	t.Helper()
	var bb bytes.Buffer
	m.marshalTo(prefix, &bb)
	result := bb.String()
	if result != resultExpected {
		t.Fatalf("unexpected marshaled metric;\ngot\n%q\nwant\n%q", result, resultExpected)
	}
}
