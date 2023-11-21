package metrics

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// PushConfig is config for pushing registered metrics to the given URL with the given Interval.
//
// URL and Interval are required fields
type PushConfig struct {
	// URL defines URL where metrics would be pushed.
	URL string
	// Interval determines the frequency of pushing metrics.
	Interval time.Duration

	// Headers contain optional http request Headers
	Headers http.Header
	// ExtraLabels may contain comma-separated list of `label="value"` labels, which will be added
	// to all the metrics before pushing them to URL.
	ExtraLabels string
	// WriteMetricsFn is a callback to write metrics to w in Prometheus text exposition format without timestamps and trailing comments.
	// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
	WriteMetricsFn func(w io.Writer)

	pushURL *url.URL
}

// Validate validates correctness of PushConfig fields
func (pc *PushConfig) Validate() error {
	if pc.Interval <= 0 {
		return fmt.Errorf("invalid Interval=%s: must be positive", pc.Interval)
	}
	if err := validateTags(pc.ExtraLabels); err != nil {
		return fmt.Errorf("invalid ExtraLabels=%q: %w", pc.ExtraLabels, err)
	}
	pu, err := url.Parse(pc.URL)
	if err != nil {
		return fmt.Errorf("cannot parse URL=%q: %w", pc.URL, err)
	}
	if pu.Scheme != "http" && pu.Scheme != "https" {
		return fmt.Errorf("unsupported scheme in URL=%q; expecting 'http' or 'https'", pc.URL)
	}
	if pu.Host == "" {
		return fmt.Errorf("missing host in URL=%q", pc.URL)
	}
	pc.pushURL = pu
	return nil
}
