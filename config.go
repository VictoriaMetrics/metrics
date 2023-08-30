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

// PushConfig is config for pushing registered metrics to the given pushURL with the given interval.
type PushConfig struct {
	// headers contain optional http request headers
	headers http.Header

	// pushURL defines URL where metrics would be pushed
	pushURL *url.URL
	// interval determines the frequency of pushing metrics
	interval time.Duration
	// extraLabels may contain comma-separated list of `label="value"` labels, which will be added
	// to all the metrics before pushing them to pushURL.
	extraLabels string
	// writeMetrics defines the function to write metrics
	writeMetrics func(w io.Writer)
}

func New(pushURL string, interval time.Duration, extraLabels string, writeMetrics func(w io.Writer), headers http.Header) (*PushConfig, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("interval must be positive; got %s", interval)
	}
	if err := validateTags(extraLabels); err != nil {
		return nil, fmt.Errorf("invalid extraLabels=%q: %w", extraLabels, err)
	}
	pu, err := url.Parse(pushURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pushURL=%q: %w", pushURL, err)
	}
	if pu.Scheme != "http" && pu.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme in pushURL=%q; expecting 'http' or 'https'", pushURL)
	}
	if pu.Host == "" {
		return nil, fmt.Errorf("missing host in pushURL=%q", pushURL)
	}

	rp := &PushConfig{
		headers:      headers,
		pushURL:      pu,
		interval:     interval,
		extraLabels:  extraLabels,
		writeMetrics: writeMetrics,
	}
	return rp, nil
}

// Push run request to the defined pushURL every interval
func (pc *PushConfig) Push() {
	pushURLRedacted := pc.pushURL.Redacted()
	cl := &http.Client{
		Timeout: pc.interval,
	}
	pushesTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_total{url=%q}`, pushURLRedacted))
	pushErrorsTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_errors_total{url=%q}`, pushURLRedacted))
	bytesPushedTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_bytes_pushed_total{url=%q}`, pushURLRedacted))
	pushDuration := pushMetrics.GetOrCreateHistogram(fmt.Sprintf(`metrics_push_duration_seconds{url=%q}`, pushURLRedacted))
	pushBlockSize := pushMetrics.GetOrCreateHistogram(fmt.Sprintf(`metrics_push_block_size_bytes{url=%q}`, pushURLRedacted))
	pushMetrics.GetOrCreateFloatCounter(fmt.Sprintf(`metrics_push_interval_seconds{url=%q}`, pushURLRedacted)).Set(pc.interval.Seconds())
	ticker := time.NewTicker(pc.interval)
	var bb bytes.Buffer
	var tmpBuf []byte
	zw := gzip.NewWriter(&bb)
	for range ticker.C {
		bb.Reset()
		pc.writeMetrics(&bb)
		if len(pc.extraLabels) > 0 {
			tmpBuf = addExtraLabels(tmpBuf[:0], bb.Bytes(), pc.extraLabels)
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

// SetHeaders can be used to set defined headers to the http request
func (pc *PushConfig) SetHeaders(req *http.Request) {
	reqHeaders := req.Header
	for key, h := range pc.headers {
		for _, s := range h {
			reqHeaders.Add(key, s)
		}
	}
}
