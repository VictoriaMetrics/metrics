// Package metrics implements Prometheus-compatible metrics for applications.
//
// This package is lightweight alternative to https://github.com/prometheus/client_golang
// with simpler API and small dependencies.
//
// Usage:
//
//     1. Register the required metrics via New* functions.
//     2. Expose them to `/metrics` page via WritePrometheus.
//     3. Update the registered metrics during application lifetime.
package metrics

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/histogram"
)

type gauge struct {
	f func() float64
}

func (g *gauge) marshalTo(prefix string, w io.Writer) {
	v := g.f()
	fmt.Fprintf(w, "%s %g\n", prefix, v)
}

// NewGauge registers gauge with the given name, which calls f
// to obtain gauge value.
//
// name must be valid Prometheus-compatible metric with possible labels.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// f must be safe for concurrent calls.
func NewGauge(name string, f func() float64) {
	g := &gauge{
		f: f,
	}
	registerMetric(name, g)
}

// Counter is a counter.
//
// It may be used as a gauge if Dec and Set are called.
type Counter struct {
	n uint64
}

// NewCounter registers and returns new counter with the given name.
//
// name must be valid Prometheus-compatible metric with possible lables.
// For instance,
//
//     * foo
//     * foo{bar="baz"}
//     * foo{bar="baz",aaa="b"}
//
// The returned counter is safe to use from concurrent goroutines.
func NewCounter(name string) *Counter {
	c := &Counter{}
	registerMetric(name, c)
	return c
}

func registerMetric(name string, m metric) {
	if err := validateMetric(name); err != nil {
		// Do not use logger.Panicf here, since it may be uninitialized yet.
		panic(fmt.Errorf("BUG: invalid metric name %q: %s", name, err))
	}
	metricsMapLock.Lock()
	ok := isRegisteredMetric(metricsMap, name)
	if !ok {
		nm := namedMetric{
			name:   name,
			metric: m,
		}
		metricsMap = append(metricsMap, nm)
	}
	metricsMapLock.Unlock()
	if ok {
		// Do not use logger.Panicf here, since it may be uninitialized yet.
		panic(fmt.Errorf("BUG: metric with name %q is already registered", name))
	}
}

// Inc increments c.
func (c *Counter) Inc() {
	atomic.AddUint64(&c.n, 1)
}

// Dec decrements c.
func (c *Counter) Dec() {
	atomic.AddUint64(&c.n, ^uint64(0))
}

// Add adds n to c.
func (c *Counter) Add(n int) {
	atomic.AddUint64(&c.n, uint64(n))
}

// Get returns the current value for c.
func (c *Counter) Get() uint64 {
	return atomic.LoadUint64(&c.n)
}

// Set sets c value to n.
func (c *Counter) Set(n uint64) {
	atomic.StoreUint64(&c.n, n)
}

// marshalTo marshals c with the given prefix to w.
func (c *Counter) marshalTo(prefix string, w io.Writer) {
	v := c.Get()
	fmt.Fprintf(w, "%s %d\n", prefix, v)
}

var (
	metricsMapLock sync.Mutex
	metricsMap     []namedMetric
)

type namedMetric struct {
	name   string
	metric metric
}

func isRegisteredMetric(mm []namedMetric, name string) bool {
	for _, nm := range mm {
		if nm.name == name {
			return true
		}
	}
	return false
}

func sortMetrics(mm []namedMetric) {
	lessFunc := func(i, j int) bool {
		return mm[i].name < mm[j].name
	}
	if !sort.SliceIsSorted(mm, lessFunc) {
		sort.Slice(mm, lessFunc)
	}
}

type metric interface {
	marshalTo(prefix string, w io.Writer)
}

// WritePrometheus writes all the registered metrics in Prometheus format to w.
//
// If exposeProcessMetrics is true, then various `go_*` metrics are exposed
// for the current process.
//
// The WritePrometheus func is usually called inside "/metrics" handler:
//
//     http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
//         metrics.WritePrometheus(w, true)
//     })
//
func WritePrometheus(w io.Writer, exposeProcessMetrics bool) {
	// Export user-defined metrics.
	metricsMapLock.Lock()
	sortMetrics(metricsMap)
	for _, nm := range metricsMap {
		nm.metric.marshalTo(nm.name, w)
	}
	metricsMapLock.Unlock()

	if !exposeProcessMetrics {
		return
	}

	// Export memory stats.
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintf(w, `go_memstats_alloc_bytes %d`+"\n", ms.Alloc)
	fmt.Fprintf(w, `go_memstats_alloc_bytes_total %d`+"\n", ms.TotalAlloc)
	fmt.Fprintf(w, `go_memstats_buck_hash_sys_bytes %d`+"\n", ms.BuckHashSys)
	fmt.Fprintf(w, `go_memstats_frees_total %d`+"\n", ms.Frees)
	fmt.Fprintf(w, `go_memstats_gc_cpu_fraction %f`+"\n", ms.GCCPUFraction)
	fmt.Fprintf(w, `go_memstats_gc_sys_bytes %d`+"\n", ms.GCSys)
	fmt.Fprintf(w, `go_memstats_heap_alloc_bytes %d`+"\n", ms.HeapAlloc)
	fmt.Fprintf(w, `go_memstats_heap_idle_bytes %d`+"\n", ms.HeapIdle)
	fmt.Fprintf(w, `go_memstats_heap_inuse_bytes %d`+"\n", ms.HeapInuse)
	fmt.Fprintf(w, `go_memstats_heap_objects %d`+"\n", ms.HeapObjects)
	fmt.Fprintf(w, `go_memstats_heap_released_bytes %d`+"\n", ms.HeapReleased)
	fmt.Fprintf(w, `go_memstats_heap_sys_bytes %d`+"\n", ms.HeapSys)
	fmt.Fprintf(w, `go_memstats_last_gc_time_seconds %f`+"\n", float64(ms.LastGC)/1e9)
	fmt.Fprintf(w, `go_memstats_lookups_total %d`+"\n", ms.Lookups)
	fmt.Fprintf(w, `go_memstats_mallocs_total %d`+"\n", ms.Mallocs)
	fmt.Fprintf(w, `go_memstats_mcache_inuse_bytes %d`+"\n", ms.MCacheInuse)
	fmt.Fprintf(w, `go_memstats_mcache_sys_bytes %d`+"\n", ms.MCacheSys)
	fmt.Fprintf(w, `go_memstats_mspan_inuse_bytes %d`+"\n", ms.MSpanInuse)
	fmt.Fprintf(w, `go_memstats_mspan_sys_bytes %d`+"\n", ms.MSpanSys)
	fmt.Fprintf(w, `go_memstats_next_gc_bytes %d`+"\n", ms.NextGC)
	fmt.Fprintf(w, `go_memstats_other_sys_bytes %d`+"\n", ms.OtherSys)
	fmt.Fprintf(w, `go_memstats_stack_inuse_bytes %d`+"\n", ms.StackInuse)
	fmt.Fprintf(w, `go_memstats_stack_sys_bytes %d`+"\n", ms.StackSys)
	fmt.Fprintf(w, `go_memstats_sys_bytes %d`+"\n", ms.Sys)

	fmt.Fprintf(w, `go_cgo_calls_count %d`+"\n", runtime.NumCgoCall())
	fmt.Fprintf(w, `go_cpu_count %d`+"\n", runtime.NumCPU())

	gcPauses := histogram.NewFast()
	for _, pauseNs := range ms.PauseNs[:] {
		gcPauses.Update(float64(pauseNs) / 1e9)
	}
	phis := []float64{0, 0.25, 0.5, 0.75, 1}
	quantiles := make([]float64, 0, len(phis))
	for i, q := range gcPauses.Quantiles(quantiles[:0], phis) {
		fmt.Fprintf(w, `go_gc_duration_seconds{quantile="%g"} %f`+"\n", phis[i], q)
	}
	fmt.Fprintf(w, `go_gc_duration_seconds_sum %f`+"\n", float64(ms.PauseTotalNs)/1e9)
	fmt.Fprintf(w, `go_gc_duration_seconds_count %d`+"\n", ms.NumGC)
	fmt.Fprintf(w, `go_gc_forced_count %d`+"\n", ms.NumForcedGC)

	fmt.Fprintf(w, `go_gomaxprocs %d`+"\n", runtime.GOMAXPROCS(0))
	fmt.Fprintf(w, `go_goroutines %d`+"\n", runtime.NumGoroutine())
	numThread, _ := runtime.ThreadCreateProfile(nil)
	fmt.Fprintf(w, `go_threads %d`+"\n", numThread)

	// Export build details.
	fmt.Fprintf(w, "go_info{version=%q} 1\n", runtime.Version())
	fmt.Fprintf(w, "go_info_ext{compiler=%q, GOARCH=%q, GOOS=%q, GOROOT=%q} 1\n",
		runtime.Compiler, runtime.GOARCH, runtime.GOOS, runtime.GOROOT())
}

var startTime = time.Now()
