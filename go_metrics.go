package metrics

import (
	"fmt"
	"io"
	"log"
	"runtime"
	runtime_metrics "runtime/metrics"

	"github.com/valyala/histogram"
)

func writeRuntimeFloat64Metric(w io.Writer, name string, sample runtime_metrics.Sample) {
	if sample.Value.Kind() == runtime_metrics.KindBad {
		// skip not supported metric
		return
	}
	fmt.Fprintf(w, "%s %g\n", name, sample.Value.Float64())
}

func writeRuntimeHistogramMetric(w io.Writer, name string, sample runtime_metrics.Sample) {
	if sample.Value.Kind() == runtime_metrics.KindBad {
		// skip not supported metric
		return
	}
	// it's unsafe to modify histogram
	h := sample.Value.Float64Histogram()
	if len(h.Buckets) == 0 {
		return
	}
	// sanity check
	if len(h.Buckets) < len(h.Counts) {
		log.Printf("ERROR: runtime_metrics.histogram: %q bad format for histogram, expected buckets to be less then counts, got: bucket %d: counts: %d", name, len(h.Buckets), len(h.Counts))
		return
	}
	var sum uint64
	// filter empty bins and convert histogram to cumulative
	for _, weight := range h.Counts {
		if weight == 0 {
			continue
		}
		sum += weight
	}
	quantile := func(phi float64) float64 {
		switch phi {
		case 0:
			return h.Buckets[0]
		case 1:
			// its guaranteed that histogram bucket has at least three values -inf, bound and + inf
			return h.Buckets[len(h.Buckets)-2]
		}
		reqValue := phi * float64(sum)
		prevIdx := 0
		cumulativeWeight := uint64(0)
		for idx, weight := range h.Counts {
			cumulativeWeight += weight
			if reqValue < float64(cumulativeWeight) {
				prevIdx = idx
				continue
			}
			break
		}

		return h.Buckets[prevIdx]
	}
	phis := []float64{0, 0.25, 0.5, 0.75, 1}
	for _, phi := range phis {
		q := quantile(phi)
		fmt.Fprintf(w, `%s{quantile="%g"} %g`+"\n", name, phi, q)
	}
}

func writeGoMetrics(w io.Writer) {
	runtimeMetricSamples := [2]runtime_metrics.Sample{
		{Name: "/sched/latencies:seconds"},
		{Name: "/sync/mutex/wait/total:seconds"},
	}
	runtime_metrics.Read(runtimeMetricSamples[:])
	writeRuntimeHistogramMetric(w, "go_sched_latency_seconds", runtimeMetricSamples[0])
	writeRuntimeFloat64Metric(w, "go_mutex_wait_total_seconds", runtimeMetricSamples[1])

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintf(w, "go_memstats_alloc_bytes %d\n", ms.Alloc)
	fmt.Fprintf(w, "go_memstats_alloc_bytes_total %d\n", ms.TotalAlloc)
	fmt.Fprintf(w, "go_memstats_buck_hash_sys_bytes %d\n", ms.BuckHashSys)
	fmt.Fprintf(w, "go_memstats_frees_total %d\n", ms.Frees)
	fmt.Fprintf(w, "go_memstats_gc_cpu_fraction %g\n", ms.GCCPUFraction)
	fmt.Fprintf(w, "go_memstats_gc_sys_bytes %d\n", ms.GCSys)

	fmt.Fprintf(w, "go_memstats_heap_alloc_bytes %d\n", ms.HeapAlloc)
	fmt.Fprintf(w, "go_memstats_heap_idle_bytes %d\n", ms.HeapIdle)
	fmt.Fprintf(w, "go_memstats_heap_inuse_bytes %d\n", ms.HeapInuse)
	fmt.Fprintf(w, "go_memstats_heap_objects %d\n", ms.HeapObjects)
	fmt.Fprintf(w, "go_memstats_heap_released_bytes %d\n", ms.HeapReleased)
	fmt.Fprintf(w, "go_memstats_heap_sys_bytes %d\n", ms.HeapSys)
	fmt.Fprintf(w, "go_memstats_last_gc_time_seconds %g\n", float64(ms.LastGC)/1e9)
	fmt.Fprintf(w, "go_memstats_lookups_total %d\n", ms.Lookups)
	fmt.Fprintf(w, "go_memstats_mallocs_total %d\n", ms.Mallocs)
	fmt.Fprintf(w, "go_memstats_mcache_inuse_bytes %d\n", ms.MCacheInuse)
	fmt.Fprintf(w, "go_memstats_mcache_sys_bytes %d\n", ms.MCacheSys)
	fmt.Fprintf(w, "go_memstats_mspan_inuse_bytes %d\n", ms.MSpanInuse)
	fmt.Fprintf(w, "go_memstats_mspan_sys_bytes %d\n", ms.MSpanSys)
	fmt.Fprintf(w, "go_memstats_next_gc_bytes %d\n", ms.NextGC)
	fmt.Fprintf(w, "go_memstats_other_sys_bytes %d\n", ms.OtherSys)
	fmt.Fprintf(w, "go_memstats_stack_inuse_bytes %d\n", ms.StackInuse)
	fmt.Fprintf(w, "go_memstats_stack_sys_bytes %d\n", ms.StackSys)
	fmt.Fprintf(w, "go_memstats_sys_bytes %d\n", ms.Sys)

	fmt.Fprintf(w, "go_cgo_calls_count %d\n", runtime.NumCgoCall())
	fmt.Fprintf(w, "go_cpu_count %d\n", runtime.NumCPU())

	gcPauses := histogram.NewFast()
	for _, pauseNs := range ms.PauseNs[:] {
		gcPauses.Update(float64(pauseNs) / 1e9)
	}
	phis := []float64{0, 0.25, 0.5, 0.75, 1}
	quantiles := make([]float64, 0, len(phis))
	for i, q := range gcPauses.Quantiles(quantiles[:0], phis) {
		fmt.Fprintf(w, `go_gc_duration_seconds{quantile="%g"} %g`+"\n", phis[i], q)
	}
	fmt.Fprintf(w, `go_gc_duration_seconds_sum %g`+"\n", float64(ms.PauseTotalNs)/1e9)
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
