package metrics_test

import (
	"bytes"
	"fmt"

	"github.com/VictoriaMetrics/metrics"
)

func ExampleSet() {
	metrics.ExposeMetadata(false)

	// Create a set with a counter
	s := metrics.NewSet()
	sc := s.NewCounter("set_counter")
	sc.Inc()
	s.NewGauge(`set_gauge{foo="bar"}`, func() float64 { return 42 })

	// Dump metrics from s.
	var bb bytes.Buffer
	s.WritePrometheus(&bb)
	fmt.Printf("set metrics:\n%s\n", bb.String())

	// Output:
	// set metrics:
	// set_counter 1
	// set_gauge{foo="bar"} 42
}

func ExampleExposeMetadata() {
	metrics.ExposeMetadata(true)
	defer metrics.ExposeMetadata(false)

	s := metrics.NewSet()

	sc := s.NewCounter("set_counter")
	sc.Inc()

	s.NewGauge(`unused_bytes{foo="bar"}`, func() float64 { return 58 })
	s.NewGauge(`used_bytes{foo="bar"}`, func() float64 { return 42 })
	s.NewGauge(`used_bytes{foo="baz"}`, func() float64 { return 43 })

	h := s.NewHistogram(`request_duration_seconds{path="/foo/bar"}`)
	h.Update(1)
	h.Update(2)

	// This summary should not exist in the output, same goes to its metadata.
	s.NewSummary(`test_summary_without_sample`)

	// This summary and metadata should exist in the output.
	// The order for summary metrics must be: quantile(s), sum, count.
	s.NewSummary("response_size_bytes").Update(1)

	// Dump metrics from s.
	var bb bytes.Buffer
	s.WritePrometheus(&bb)
	fmt.Printf("set metrics:\n%s\n", bb.String())

	// Output:
	// set metrics:
	// # HELP request_duration_seconds
	// # TYPE request_duration_seconds histogram
	// request_duration_seconds_bucket{path="/foo/bar",vmrange="8.799e-01...1.000e+00"} 1
	// request_duration_seconds_bucket{path="/foo/bar",vmrange="1.896e+00...2.154e+00"} 1
	// request_duration_seconds_sum{path="/foo/bar"} 3
	// request_duration_seconds_count{path="/foo/bar"} 2
	// # HELP response_size_bytes
	// # TYPE response_size_bytes summary
	// response_size_bytes_sum 1
	// response_size_bytes_count 1
	// response_size_bytes{quantile="0.5"} 1
	// response_size_bytes{quantile="0.9"} 1
	// response_size_bytes{quantile="0.97"} 1
	// response_size_bytes{quantile="0.99"} 1
	// response_size_bytes{quantile="1"} 1
	// # HELP set_counter
	// # TYPE set_counter counter
	// set_counter 1
	// # HELP unused_bytes
	// # TYPE unused_bytes gauge
	// unused_bytes{foo="bar"} 58
	// # HELP used_bytes
	// # TYPE used_bytes gauge
	// used_bytes{foo="bar"} 42
	// used_bytes{foo="baz"} 43
}
