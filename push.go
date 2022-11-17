package metrics

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"compress/gzip"
)

// InitPushProcessMetrics sets up periodic push for 'process_*' metrics to the given pushURL with the given interval.
//
// extraLabels may contain comma-separated list of `label="value"` labels, which will be added
// to all the metrics before pushing them to pushURL.
//
// The metrics are pushed to pushURL in Prometheus text exposition format.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPushProcessMetrics multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func InitPushProcessMetrics(pushURL string, interval time.Duration, extraLabels string) error {
	writeMetrics := func(w io.Writer) {
		WriteProcessMetrics(w)
	}
	return InitPushExt(pushURL, interval, extraLabels, writeMetrics)
}

// InitPush sets up periodic push for globally registered metrics to the given pushURL with the given interval.
//
// extraLabels may contain comma-separated list of `label="value"` labels, which will be added
// to all the metrics before pushing them to pushURL.
//
// If pushProcessMetrics is set to true, then 'process_*' metrics are also pushed to pushURL.
//
// The metrics are pushed to pushURL in Prometheus text exposition format.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPush multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func InitPush(pushURL string, interval time.Duration, extraLabels string, pushProcessMetrics bool) error {
	writeMetrics := func(w io.Writer) {
		WritePrometheus(w, pushProcessMetrics)
	}
	return InitPushExt(pushURL, interval, extraLabels, writeMetrics)
}

// InitPush sets up periodic push for metrics from s to the given pushURL with the given interval.
//
// extraLabels may contain comma-separated list of `label="value"` labels, which will be added
// to all the metrics before pushing them to pushURL.
//
// / The metrics are pushed to pushURL in Prometheus text exposition format.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPush multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func (s *Set) InitPush(pushURL string, interval time.Duration, extraLabels string) error {
	writeMetrics := func(w io.Writer) {
		s.WritePrometheus(w)
	}
	s.pushUrl = pushURL
	return InitPushExt(pushURL, interval, extraLabels, writeMetrics)
}

// Close send a last metric push to the a metrics.Sets{}'s pushURL and stops the pushing of metrics for that set.
//
// Is it recommended to call metrics.Set{}.Close() before a Set{} is removed from memory so all metrics are flushed one last time.
//
// Close() will always return a nil err
func (s *Set) Close() error {
	v, ok := pushUrlChanMap.LoadAndDelete(s.pushUrl)
	if !ok {
		return nil
	}
	closeWG.Add(1)
	v.(*chanMutex).close()
	closeWG.Wait()
	return nil
}

// pushUrlChanMap is a set of channels that signal if metrics should be collected.
var pushUrlChanMap = &sync.Map{}
var closeWG = &sync.WaitGroup{}

type chanMutex struct {
	c      chan struct{}
	m      *sync.Mutex
	closed bool
}

func (c *chanMutex) close() {
	c.m.Lock()
	defer c.m.Unlock()
	close(c.c)
	c.closed = true
}

func (c *chanMutex) send() bool {
	c.m.Lock()
	defer c.m.Unlock()
	if c.closed {
		return false
	}
	c.c <- struct{}{}
	return true
}

func timeTrigger(interval time.Duration, c *chanMutex) {
	go func() {
		ticker := time.NewTicker(interval)
		for range ticker.C {
			if !c.send() {
				ticker.Stop()
			}
		}
	}()
}

// Close flushes all metrics for both global and non-global metrics.Set{} to their respective pushURL's and stops
// all timers.
//
// Is it recommended to call Close() before a job is removed from memory so all metrics are flushed one last time to pushURL's
//
// Close() will always return a nil err
func Close() error {
	pushUrlChanMap.Range(func(k, v interface{}) bool {
		closeWG.Add(1)
		v.(*chanMutex).close()
		pushUrlChanMap.Delete(k)
		return true
	})
	closeWG.Wait()
	return nil
}

// InitPushExt sets up periodic push for metrics obtained by calling writeMetrics with the given interval.
//
// extraLabels may contain comma-separated list of `label="value"` labels, which will be added
// to all the metrics before pushing them to pushURL.
//
// The writeMetrics callback must write metrics to w in Prometheus text exposition format without timestamps and trailing comments.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPushExt multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
//
// It is OK calling InitPushExt multiple times with different writeMetrics -
// in this case all the metrics generated by writeMetrics callbacks are writte to pushURL.
func InitPushExt(pushURL string, interval time.Duration, extraLabels string, writeMetrics func(w io.Writer)) (err error) {
	defer func() {
		if err != nil {
			pushUrlChanMap.Delete(pushURL)
		}
	}()

	if interval <= 0 {
		return fmt.Errorf("interval must be positive; got %s", interval)
	}
	if err := validateTags(extraLabels); err != nil {
		return fmt.Errorf("invalid extraLabels=%q: %w", extraLabels, err)
	}
	pu, err := url.Parse(pushURL)
	if err != nil {
		return fmt.Errorf("cannot parse pushURL=%q: %w", pushURL, err)
	}
	if pu.Scheme != "http" && pu.Scheme != "https" {
		return fmt.Errorf("unsupported scheme in pushURL=%q; expecting 'http' or 'https'", pushURL)
	}
	if pu.Host == "" {
		return fmt.Errorf("missing host in pushURL=%q", pushURL)
	}

	// record the signal for sending metrics
	cm := &chanMutex{c: make(chan struct{}), m: &sync.Mutex{}}
	if _, ok := pushUrlChanMap.Load(pushURL); ok {
		return nil
	}
	pushUrlChanMap.Store(pushURL, cm)
	timeTrigger(interval, cm)

	pushURLRedacted := pu.Redacted()
	c := &http.Client{
		Timeout: interval,
	}
	pushesTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_total{url=%q}`, pushURLRedacted))
	pushErrorsTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_errors_total{url=%q}`, pushURLRedacted))
	bytesPushedTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_bytes_pushed_total{url=%q}`, pushURLRedacted))
	pushDuration := pushMetrics.GetOrCreateHistogram(fmt.Sprintf(`metrics_push_duration_seconds{url=%q}`, pushURLRedacted))
	pushBlockSize := pushMetrics.GetOrCreateHistogram(fmt.Sprintf(`metrics_push_block_size_bytes{url=%q}`, pushURLRedacted))
	pushCloseTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_close_total{url=%q}`, pushURLRedacted))
	pushMetrics.GetOrCreateFloatCounter(fmt.Sprintf(`metrics_push_interval_seconds{url=%q}`, pushURLRedacted)).Set(interval.Seconds())
	go func() {
		var bb bytes.Buffer
		var tmpBuf []byte
		zw := gzip.NewWriter(&bb)
		for {
			select {
			case _, ok := <-cm.c:
				{
					if !ok {
						pushCloseTotal.Inc()
					}

					bb.Reset()
					writeMetrics(&bb)
					if len(extraLabels) > 0 {
						tmpBuf = addExtraLabels(tmpBuf[:0], bb.Bytes(), extraLabels)
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
					req, err := http.NewRequest("GET", pushURL, &bb)
					if err != nil {
						panic(fmt.Errorf("BUG: metrics.push: cannot initialize request for metrics push to %q: %w", pushURLRedacted, err))
					}
					req.Header.Set("Content-Type", "text/plain")
					req.Header.Set("Content-Encoding", "gzip")
					startTime := time.Now()
					resp, err := c.Do(req)
					pushDuration.UpdateDuration(startTime)
					if err != nil {
						log.Printf("ERROR: metrics.push: cannot push metrics to %q: %s", pushURLRedacted, err)
						pushErrorsTotal.Inc()
						continue
					}
					if resp.StatusCode/100 != 2 {
						body, _ := ioutil.ReadAll(resp.Body)
						_ = resp.Body.Close()
						log.Printf("ERROR: metrics.push: unexpected status code in response from %q: %d; expecting 2xx; response body: %q",
							pushURLRedacted, resp.StatusCode, body)
						pushErrorsTotal.Inc()
						continue
					}
					_ = resp.Body.Close()
					if !ok {
						closeWG.Done()
						return
					}
				}
			}
		}
	}()
	return nil
}

var pushMetrics = NewSet()

func writePushMetrics(w io.Writer) {
	pushMetrics.WritePrometheus(w)
}

func addExtraLabels(dst, src []byte, extraLabels string) []byte {
	for len(src) > 0 {
		var line []byte
		n := bytes.IndexByte(src, '\n')
		if n >= 0 {
			line = src[:n]
			src = src[n+1:]
		} else {
			line = src
			src = nil
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			// Skip empy lines
			continue
		}
		if bytes.HasPrefix(line, bashBytes) {
			// Copy comments as is
			dst = append(dst, line...)
			dst = append(dst, '\n')
			continue
		}
		n = bytes.IndexByte(line, '{')
		if n >= 0 {
			dst = append(dst, line[:n+1]...)
			dst = append(dst, extraLabels...)
			dst = append(dst, ',')
			dst = append(dst, line[n+1:]...)
		} else {
			n = bytes.LastIndexByte(line, ' ')
			if n < 0 {
				panic(fmt.Errorf("BUG: missing whitespace between metric name and metric value in Prometheus text exposition line %q", line))
			}
			dst = append(dst, line[:n]...)
			dst = append(dst, '{')
			dst = append(dst, extraLabels...)
			dst = append(dst, '}')
			dst = append(dst, line[n:]...)
		}
		dst = append(dst, '\n')
	}
	return dst
}

var bashBytes = []byte("#")
