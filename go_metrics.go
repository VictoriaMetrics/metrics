package metrics

import (
	"fmt"
	"io"
	"runtime"

	"github.com/valyala/histogram"
)

func writeGoMetrics(w io.Writer, writeType bool) {
	t := func(name string, t metricType) {
		if writeType {
			writeTypeTo(name, t, w)
		}
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t("go_memstats_alloc_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_alloc_bytes %d\n", ms.Alloc)
	t("go_memstats_alloc_bytes_total", gaugeType)
	fmt.Fprintf(w, "go_memstats_alloc_bytes_total %d\n", ms.TotalAlloc)
	t("go_memstats_buck_hash_sys_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_buck_hash_sys_bytes %d\n", ms.BuckHashSys)
	t("go_memstats_frees_total", counterType)
	fmt.Fprintf(w, "go_memstats_frees_total %d\n", ms.Frees)
	t("go_memstats_gc_cpu_fraction", gaugeType)
	fmt.Fprintf(w, "go_memstats_gc_cpu_fraction %g\n", ms.GCCPUFraction)
	t("go_memstats_gc_sys_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_gc_sys_bytes %d\n", ms.GCSys)
	t("go_memstats_heap_alloc_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_heap_alloc_bytes %d\n", ms.HeapAlloc)
	t("go_memstats_heap_idle_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_heap_idle_bytes %d\n", ms.HeapIdle)
	t("go_memstats_heap_inuse_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_heap_inuse_bytes %d\n", ms.HeapInuse)
	t("go_memstats_heap_objects", gaugeType)
	fmt.Fprintf(w, "go_memstats_heap_objects %d\n", ms.HeapObjects)
	t("go_memstats_heap_released_bytes", counterType)
	fmt.Fprintf(w, "go_memstats_heap_released_bytes %d\n", ms.HeapReleased)
	t("go_memstats_heap_sys_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_heap_sys_bytes %d\n", ms.HeapSys)
	t("go_memstats_last_gc_time_seconds", gaugeType)
	fmt.Fprintf(w, "go_memstats_last_gc_time_seconds %g\n", float64(ms.LastGC)/1e9)
	t("go_memstats_lookups_total", counterType)
	fmt.Fprintf(w, "go_memstats_lookups_total %d\n", ms.Lookups)
	t("go_memstats_mallocs_total", counterType)
	fmt.Fprintf(w, "go_memstats_mallocs_total %d\n", ms.Mallocs)
	t("go_memstats_mcache_inuse_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_mcache_inuse_bytes %d\n", ms.MCacheInuse)
	t("go_memstats_mcache_sys_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_mcache_sys_bytes %d\n", ms.MCacheSys)
	t("go_memstats_mspan_inuse_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_mspan_inuse_bytes %d\n", ms.MSpanInuse)
	t("go_memstats_mspan_sys_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_mspan_sys_bytes %d\n", ms.MSpanSys)
	t("go_memstats_next_gc_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_next_gc_bytes %d\n", ms.NextGC)
	t("go_memstats_other_sys_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_other_sys_bytes %d\n", ms.OtherSys)
	t("go_memstats_stack_inuse_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_stack_inuse_bytes %d\n", ms.StackInuse)
	t("go_memstats_stack_sys_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_stack_sys_bytes %d\n", ms.StackSys)
	t("go_memstats_sys_bytes", gaugeType)
	fmt.Fprintf(w, "go_memstats_sys_bytes %d\n", ms.Sys)
	t("go_cgo_calls_count", counterType)
	fmt.Fprintf(w, "go_cgo_calls_count %d\n", runtime.NumCgoCall())
	t("go_cpu_count", gaugeType)
	fmt.Fprintf(w, "go_cpu_count %d\n", runtime.NumCPU())

	gcPauses := histogram.NewFast()
	for _, pauseNs := range ms.PauseNs[:] {
		gcPauses.Update(float64(pauseNs) / 1e9)
	}
	phis := []float64{0, 0.25, 0.5, 0.75, 1}
	quantiles := make([]float64, 0, len(phis))
	t("go_gc_duration_seconds", summaryType)
	for i, q := range gcPauses.Quantiles(quantiles[:0], phis) {
		fmt.Fprintf(w, `go_gc_duration_seconds{quantile="%g"} %g`+"\n", phis[i], q)
	}
	t("go_gc_duration_seconds_sum", gaugeType)
	fmt.Fprintf(w, `go_gc_duration_seconds_sum %g`+"\n", float64(ms.PauseTotalNs)/1e9)
	t("go_gc_duration_seconds_count", counterType)
	fmt.Fprintf(w, `go_gc_duration_seconds_count %d`+"\n", ms.NumGC)
	t("go_gc_forced_count", counterType)
	fmt.Fprintf(w, `go_gc_forced_count %d`+"\n", ms.NumForcedGC)

	t("go_gomaxprocs", gaugeType)
	fmt.Fprintf(w, `go_gomaxprocs %d`+"\n", runtime.GOMAXPROCS(0))
	t("go_goroutines", gaugeType)
	fmt.Fprintf(w, `go_goroutines %d`+"\n", runtime.NumGoroutine())
	numThread, _ := runtime.ThreadCreateProfile(nil)
	t("go_threads", gaugeType)
	fmt.Fprintf(w, `go_threads %d`+"\n", numThread)

	// Export build details.
	t("go_info", untypedType)
	fmt.Fprintf(w, "go_info{version=%q} 1\n", runtime.Version())
	t("go_info_ext", untypedType)
	fmt.Fprintf(w, "go_info_ext{compiler=%q, GOARCH=%q, GOOS=%q, GOROOT=%q} 1\n",
		runtime.Compiler, runtime.GOARCH, runtime.GOOS, runtime.GOROOT())
}
