package metrics

import (
	"bytes"
	"fmt"
	"io"
	"time"
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

// InitPushWithConfig sets up periodic push for globally registered metrics to the given pushURL with the given interval
// defined in the PushConfig
//
// It is OK calling InitPushWithConfig multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func InitPushWithConfig(pushConfig *PushConfig, pushProcessMetrics bool) error {
	if err := pushConfig.Validate(); err != nil {
		return err
	}
	pushConfig.WriteMetrics = func(w io.Writer) {
		WritePrometheus(w, pushProcessMetrics)
	}
	go pushConfig.Push()
	return nil
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
	return InitPushExt(pushURL, interval, extraLabels, writeMetrics)
}

// InitPushWithConfig sets up periodic push for globally registered metrics to the given PushURL with the given Interval
// defined in the PushConfig
//
// It is OK calling InitPushWithConfig multiple times with different PushURL -
// in this case metrics are pushed to all the provided PushURL urls.
func (s *Set) InitPushWithConfig(pushConfig *PushConfig) error {
	if err := pushConfig.Validate(); err != nil {
		return err
	}
	pushConfig.WriteMetrics = func(w io.Writer) {
		s.WritePrometheus(w)
	}
	go pushConfig.Push()
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
// in this case all the metrics generated by writeMetrics callbacks are written to pushURL.
func InitPushExt(pushURL string, interval time.Duration, extraLabels string, writeMetrics func(w io.Writer)) error {
	pc := &PushConfig{
		PushURL:      pushURL,
		Interval:     interval,
		ExtraLabels:  extraLabels,
		WriteMetrics: writeMetrics,
	}
	return InitPushExtWithConfig(pc)
}

// InitPushExtWithConfig sets up periodic push for metrics obtained by calling writeMetrics with the given interval
// defined in the PushConfig.
//
// It is OK calling InitPushExtWithConfig multiple times with different writeMetrics -
// in this case all the metrics generated by writeMetrics callbacks are written to PushURL.
func InitPushExtWithConfig(pushConfig *PushConfig) error {
	if err := pushConfig.Validate(); err != nil {
		return err
	}
	go pushConfig.Push()
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
