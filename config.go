package metrics

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// PushConfig is config for pushing registered metrics to the given PushURL with the given Interval.
//
// PushURL and Interval are required fields
type PushConfig struct {
	// PushURL defines URL where metrics would be pushed.
	PushURL string
	// Interval determines the frequency of pushing metrics.
	Interval time.Duration

	// Headers contain optional http request Headers
	Headers http.Header
	// ExtraLabels may contain comma-separated list of `label="value"` labels, which will be added
	// to all the metrics before pushing them to PushURL.
	ExtraLabels string

	pushURL *url.URL
}

// Validate checks the defined fields and returns error if some of the field
// has incorrect value
func (pc *PushConfig) Validate() error {
	if pc.Interval <= 0 {
		return fmt.Errorf("interval must be positive; got %s", pc.Interval)
	}
	if err := validateTags(pc.ExtraLabels); err != nil {
		return fmt.Errorf("invalid ExtraLabels=%q: %w", pc.ExtraLabels, err)
	}
	pu, err := parseURL(pc.PushURL)
	if err != nil {
		return fmt.Errorf("field PushURL not valid: %w", err)
	}

	pc.pushURL = pu
	return nil
}

// Push run request to the defined PushURL every Interval
func (pc *PushConfig) Push(writeMetrics func(w io.Writer)) {
	if writeMetrics == nil {
		panic(fmt.Errorf("write metrics function not defined"))
	}
	pushURLRedacted := pc.pushURL.Redacted()
	// by default set interval to one second
	if pc.Interval == 0 {
		pc.Interval = time.Second
	}
	cl := &http.Client{
		Timeout: pc.Interval,
	}
	pushesTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_total{url=%q}`, pushURLRedacted))
	pushErrorsTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_errors_total{url=%q}`, pushURLRedacted))
	bytesPushedTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_bytes_pushed_total{url=%q}`, pushURLRedacted))
	pushDuration := pushMetrics.GetOrCreateHistogram(fmt.Sprintf(`metrics_push_duration_seconds{url=%q}`, pushURLRedacted))
	pushBlockSize := pushMetrics.GetOrCreateHistogram(fmt.Sprintf(`metrics_push_block_size_bytes{url=%q}`, pushURLRedacted))
	pushMetrics.GetOrCreateFloatCounter(fmt.Sprintf(`metrics_push_interval_seconds{url=%q}`, pushURLRedacted)).Set(pc.Interval.Seconds())
	ticker := time.NewTicker(pc.Interval)
	var bb bytes.Buffer
	var tmpBuf []byte
	zw := gzip.NewWriter(&bb)
	for range ticker.C {
		bb.Reset()
		writeMetrics(&bb)
		if len(pc.ExtraLabels) > 0 {
			tmpBuf = addExtraLabels(tmpBuf[:0], bb.Bytes(), pc.ExtraLabels)
			bb.Reset()
			if _, err := bb.Write(tmpBuf); err != nil {
				panic(fmt.Errorf("BUG: cannot write %d bytes to bytes.Buffer: %s", len(tmpBuf), err))
			}
		}
		tmpBuf = append(tmpBuf[:0], bb.Bytes()...)
		bb.Reset()
		zw.Reset(&bb)
		if _, err := zw.Write(tmpBuf); err != nil {
			panic(fmt.Errorf("BUG: cannot write %d bytes to gzip writer: %s", len(tmpBuf), err))
		}
		if err := zw.Close(); err != nil {
			panic(fmt.Errorf("BUG: cannot flush metrics to gzip writer: %s", err))
		}
		pushesTotal.Inc()
		blockLen := bb.Len()
		bytesPushedTotal.Add(blockLen)
		pushBlockSize.Update(float64(blockLen))
		u := pc.pushURL.String()
		req, err := http.NewRequest("GET", u, &bb)
		if err != nil {
			panic(fmt.Errorf("BUG: metrics.push: cannot initialize request for metrics push to %q: %w", pushURLRedacted, err))
		}
		pc.SetHeaders(req)
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Content-Encoding", "gzip")
		startTime := time.Now()
		resp, err := cl.Do(req)
		pushDuration.UpdateDuration(startTime)
		if err != nil {
			log.Printf("ERROR: metrics.push: cannot push metrics to %q: %s", pushURLRedacted, err)
			pushErrorsTotal.Inc()
			continue
		}
		if resp.StatusCode/100 != 2 {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			log.Printf("ERROR: metrics.push: unexpected status code in response from %q: %d; expecting 2xx; response body: %q",
				pushURLRedacted, resp.StatusCode, body)
			pushErrorsTotal.Inc()
			continue
		}
		_ = resp.Body.Close()
	}
}

// SetHeaders can be used to set defined Headers to the http request
func (pc *PushConfig) SetHeaders(req *http.Request) {
	reqHeaders := req.Header
	for key, h := range pc.Headers {
		for _, s := range h {
			reqHeaders.Add(key, s)
		}
	}
}

func parseURL(u string) (*url.URL, error) {
	if u == "" {
		return nil, fmt.Errorf("url cannot br empty")
	}
	pu, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("cannot parse url=%q: %w", u, err)
	}
	if pu.Scheme != "http" && pu.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme in url=%q; expecting 'http' or 'https'", u)
	}
	if pu.Host == "" {
		return nil, fmt.Errorf("missing host in url=%q", u)
	}
	return pu, nil
}
