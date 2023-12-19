package metrics

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWriteMetrics(t *testing.T) {
	t.Run("gauge_uint64", func(t *testing.T) {
		var bb bytes.Buffer

		WriteGaugeUint64(&bb, "foo", 123)
		sExpected := "foo 123\n"
		if s := bb.String(); s != sExpected {
			t.Fatalf("unexpected value; got\n%s\nwant\n%s", s, sExpected)
		}

		ExposeMetadata(true)
		bb.Reset()
		WriteGaugeUint64(&bb, "foo", 123)
		sExpected = "# HELP foo\n# TYPE foo gauge\nfoo 123\n"
		ExposeMetadata(false)
		if s := bb.String(); s != sExpected {
			t.Fatalf("unexpected value; got\n%s\nwant\n%s", s, sExpected)
		}
	})
	t.Run("gauge_float64", func(t *testing.T) {
		var bb bytes.Buffer

		WriteGaugeFloat64(&bb, "foo", 1.23)
		sExpected := "foo 1.23\n"
		if s := bb.String(); s != sExpected {
			t.Fatalf("unexpected value; got\n%s\nwant\n%s", s, sExpected)
		}

		ExposeMetadata(true)
		bb.Reset()
		WriteGaugeFloat64(&bb, "foo", 1.23)
		sExpected = "# HELP foo\n# TYPE foo gauge\nfoo 1.23\n"
		ExposeMetadata(false)
		if s := bb.String(); s != sExpected {
			t.Fatalf("unexpected value; got\n%s\nwant\n%s", s, sExpected)
		}
	})
	t.Run("counter_uint64", func(t *testing.T) {
		var bb bytes.Buffer

		WriteCounterUint64(&bb, "foo_total", 123)
		sExpected := "foo_total 123\n"
		if s := bb.String(); s != sExpected {
			t.Fatalf("unexpected value; got\n%s\nwant\n%s", s, sExpected)
		}

		ExposeMetadata(true)
		bb.Reset()
		WriteCounterUint64(&bb, "foo_total", 123)
		sExpected = "# HELP foo_total\n# TYPE foo_total counter\nfoo_total 123\n"
		ExposeMetadata(false)
		if s := bb.String(); s != sExpected {
			t.Fatalf("unexpected value; got\n%s\nwant\n%s", s, sExpected)
		}
	})
	t.Run("counter_float64", func(t *testing.T) {
		var bb bytes.Buffer

		WriteCounterFloat64(&bb, "foo_total", 1.23)
		sExpected := "foo_total 1.23\n"
		if s := bb.String(); s != sExpected {
			t.Fatalf("unexpected value; got\n%s\nwant\n%s", s, sExpected)
		}

		ExposeMetadata(true)
		bb.Reset()
		WriteCounterFloat64(&bb, "foo_total", 1.23)
		sExpected = "# HELP foo_total\n# TYPE foo_total counter\nfoo_total 1.23\n"
		ExposeMetadata(false)
		if s := bb.String(); s != sExpected {
			t.Fatalf("unexpected value; got\n%s\nwant\n%s", s, sExpected)
		}
	})
}

func TestGetDefaultSet(t *testing.T) {
	s := GetDefaultSet()
	if s != defaultSet {
		t.Fatalf("GetDefaultSet must return defaultSet=%p, but returned %p", defaultSet, s)
	}
}

func TestUnregisterAllMetrics(t *testing.T) {
	for j := 0; j < 3; j++ {
		for i := 0; i < 10; i++ {
			_ = NewCounter(fmt.Sprintf("counter_%d", i))
			_ = NewSummary(fmt.Sprintf("summary_%d", i))
			_ = NewHistogram(fmt.Sprintf("histogram_%d", i))
			_ = NewGauge(fmt.Sprintf("gauge_%d", i), func() float64 { return 0 })
		}
		if mns := ListMetricNames(); len(mns) == 0 {
			t.Fatalf("unexpected empty list of metrics on iteration %d", j)
		}
		UnregisterAllMetrics()
		if mns := ListMetricNames(); len(mns) != 0 {
			t.Fatalf("unexpected metric names after UnregisterAllMetrics call on iteration %d: %q", j, mns)
		}
	}
}

func TestRegisterUnregisterSet(t *testing.T) {
	const metricName = "metric_from_set"
	const metricValue = 123
	s := NewSet()
	c := s.NewCounter(metricName)
	c.Set(metricValue)

	RegisterSet(s)
	var bb bytes.Buffer
	WritePrometheus(&bb, false)
	data := bb.String()
	expectedLine := fmt.Sprintf("%s %d\n", metricName, metricValue)
	if !strings.Contains(data, expectedLine) {
		t.Fatalf("missing %q in\n%s", expectedLine, data)
	}

	UnregisterSet(s)
	bb.Reset()
	WritePrometheus(&bb, false)
	data = bb.String()
	if strings.Contains(data, expectedLine) {
		t.Fatalf("unepected %q in\n%s", expectedLine, data)
	}
}

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
