package metrics

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestAvailableCPUCoresQuota(t *testing.T) {
	v2Mount := "30 23 0:26 / /cg rw - cgroup2 cgroup rw\n"
	v1Mount := "31 23 0:27 / /cpu rw - cgroup cgroup rw,cpu,cpuacct\n"
	for _, tc := range []struct {
		name, membership, mounts string
		files                    map[string]string
		want                     float64
	}{
		{"v2 fractional", "0::/app", v2Mount, map[string]string{"/cg/app/cpu.max": "150000 100000\n"}, 1.5},
		{"v1 fractional", "2:cpu,cpuacct:/app", v1Mount, map[string]string{"/cpu/app/cpu.cfs_quota_us": "50000\n", "/cpu/app/cpu.cfs_period_us": "100000\n"}, 0.5},
		{"v2 unlimited", "0::/app", v2Mount, map[string]string{"/cg/app/cpu.max": "max 100000"}, 8},
		{"v1 unlimited", "2:cpu:/app", v1Mount, map[string]string{"/cpu/app/cpu.cfs_quota_us": "-1", "/cpu/app/cpu.cfs_period_us": "100000"}, 8},
		{"parent quota", "0::/group/app", v2Mount, map[string]string{"/cg/group/app/cpu.max": "max 100000", "/cg/group/cpu.max": "150000 100000"}, 1.5},
		{"minimum quota", "0::/group/app", v2Mount, map[string]string{"/cg/group/app/cpu.max": "300000 100000", "/cg/group/cpu.max": "150000 100000"}, 1.5},
		{"v1 parent", "2:cpu:/app", v1Mount, map[string]string{"/cpu/cpu.cfs_quota_us": "250000", "/cpu/cpu.cfs_period_us": "100000"}, 2.5},
		{"mount root", "0::/group/app", "30 23 0:26 /group /cg rw - cgroup2 cgroup rw", map[string]string{"/cg/app/cpu.max": "250000 100000", "/cpu.max": "50000 100000"}, 2.5},
		{"namespace root", "0::/", v2Mount, map[string]string{"/cg/cpu.max": "50000 100000"}, 0.5},
		{"unrelated mount root", "0::/other/app", "30 23 0:26 /group /cg rw - cgroup2 cgroup rw", map[string]string{"/cg/cpu.max": "50000 100000"}, 8},
		{"hybrid", "0::/app\n2:cpu,cpuacct:/app", v2Mount + v1Mount, map[string]string{"/cpu/app/cpu.cfs_quota_us": "150000", "/cpu/app/cpu.cfs_period_us": "100000"}, 1.5},
		{"escaped mount", "0::/app", "30 23 0:26 / /my\\040cg rw - cgroup2 cgroup rw", map[string]string{"/my cg/app/cpu.max": "150000 100000"}, 1.5},
		{"quota above host count", "0::/app", v2Mount, map[string]string{"/cg/app/cpu.max": "1600000 100000"}, 16},
		{"missing quota", "0::/app", v2Mount, nil, 8},
		{"missing membership", "", v2Mount, nil, 8},
		{"missing mounts", "0::/app", "", nil, 8},
		{"malformed mounts", "0::/app", "x - cgroup2\n", nil, 8},
		{"zero period", "0::/app", v2Mount, map[string]string{"/cg/app/cpu.max": "100000 0"}, 8},
		{"malformed quota", "0::/app", v2Mount, map[string]string{"/cg/app/cpu.max": "NaN 100000"}, 8},
		{"negative quota", "0::/app", v2Mount, map[string]string{"/cg/app/cpu.max": "-2 100000"}, 8},
		{"missing period", "0::/app", v2Mount, map[string]string{"/cg/app/cpu.max": "100000"}, 8},
		{"zero quota", "0::/app", v2Mount, map[string]string{"/cg/app/cpu.max": "0 100000"}, 8},
		{"v1 bad period", "2:cpu:/app", v1Mount, map[string]string{"/cpu/app/cpu.cfs_quota_us": "150000", "/cpu/app/cpu.cfs_period_us": "0"}, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readFile := func(name string) ([]byte, error) {
				data, ok := tc.files[name]
				if !ok {
					return nil, os.ErrNotExist
				}
				return []byte(data), nil
			}
			if got := availableCPUCoresFromCgroup(tc.membership, tc.mounts, readFile, 8); got != tc.want {
				t.Fatalf("got %g cores; want %g", got, tc.want)
			}
		})
	}
}

func TestAvailableCPUCoresQuotaChanges(t *testing.T) {
	quota := "150000 100000"
	readFile := func(name string) ([]byte, error) {
		if name == "/cg/cpu.max" {
			return []byte(quota), nil
		}
		return nil, os.ErrNotExist
	}
	for _, want := range []float64{1.5, 0.5} {
		quota = fmt.Sprintf("%.0f 100000", want*100000)
		got := availableCPUCoresFromCgroup("0::/", "30 23 0:26 / /cg rw - cgroup2 cgroup rw", readFile, 8)
		if got != want {
			t.Fatalf("got %g cores after quota change; want %g", got, want)
		}
	}
}

func TestProcessAvailableCPUCoresExport(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	for _, tc := range []struct {
		name  string
		write func(io.Writer)
		want  bool
	}{
		{"process", WriteProcessMetrics, true},
		{"proc", WriteProcMetrics, true},
		{"prometheus enabled", func(w io.Writer) { WritePrometheus(w, true) }, true},
		{"prometheus disabled", func(w io.Writer) { WritePrometheus(w, false) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			tc.write(&b)
			count := 0
			for line := range strings.SplitSeq(b.String(), "\n") {
				if value, ok := strings.CutPrefix(line, "process_available_cpu_cores "); ok {
					count++
					v, err := strconv.ParseFloat(value, 64)
					if err != nil || !(v > 0) {
						t.Fatalf("invalid available cores %q", value)
					}
					if runtime.GOOS != "linux" && v != float64(runtime.NumCPU()) {
						t.Fatalf("got %g cores; want NumCPU=%d", v, runtime.NumCPU())
					}
				}
			}
			wantCount := 0
			if tc.want {
				wantCount = 1
			}
			if count != wantCount {
				t.Fatalf("got %d samples; want %d", count, wantCount)
			}
			if runtime.GOMAXPROCS(0) != 1 {
				t.Fatal("collection modified GOMAXPROCS")
			}
		})
	}
}

func TestProcessAvailableCPUCoresMetadata(t *testing.T) {
	ExposeMetadata(true)
	t.Cleanup(func() { ExposeMetadata(false) })
	var b bytes.Buffer
	WriteProcMetrics(&b)
	if !strings.Contains(b.String(), "# TYPE process_available_cpu_cores gauge\n") {
		t.Fatal("missing gauge metadata")
	}
}
