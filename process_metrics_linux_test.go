package metrics

import (
	"testing"
)

func TestGetMaxFilesLimit(t *testing.T) {
	f := func(want uint64, path string, wantErr bool) {
		t.Helper()
		got, err := getMaxFilesLimit(path)
		if err != nil && !wantErr {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Fatalf("unexpected result: %d, want: %d at getMaxFilesLimit", got, want)
		}

	}
	f(1024, "testdata/limits", false)
	f(0, "testdata/bad_path", true)
	f(0, "testdata/limits_bad", true)
}

func TestGetOpenFDsCount(t *testing.T) {
	f := func(want uint64, path string, wantErr bool) {
		t.Helper()
		got, err := getOpenFDsCount(path)
		if (err != nil && !wantErr) || (err == nil && wantErr) {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Fatalf("unexpected result: %d, want: %d at getOpenFDsCount", got, want)
		}
	}
	f(5, "testdata/fd/", false)
	f(0, "testdata/fd/0", true)
	f(0, "testdata/limits", true)
}

func TestGetMemStats(t *testing.T) {
	f := func(want memStats, path string, wantErr bool) {
		t.Helper()
		got, err := getMemStats(path)
		if (err != nil && !wantErr) || (err == nil && wantErr) {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil && *got != want {
			t.Fatalf("unexpected result: %d, want: %d at getMemStats", *got, want)
		}
	}
	f(memStats{vmPeak: 2130489344, rssPeak: 200679424, rssAnon: 121602048, rssFile: 11362304}, "testdata/status", false)
	f(memStats{}, "testdata/status_bad", true)
}

func TestGetCgroupPaths(t *testing.T) {
	f := func(cgroupData, v2Mountpoint, wantV2Path, wantCPUStatPath string) {
		t.Helper()
		v2Path, cpuStatPath := getCgroupPaths(cgroupData, v2Mountpoint)
		if v2Path != wantV2Path {
			t.Fatalf("unexpected v2 path: %q, want: %q", v2Path, wantV2Path)
		}
		if cpuStatPath != wantCPUStatPath {
			t.Fatalf("unexpected cpu.stat path: %q, want: %q", cpuStatPath, wantCPUStatPath)
		}
	}

	// Unified cgroup v2 mounted at /sys/fs/cgroup.
	f("0::/user.slice/session-1.scope\n", "/sys/fs/cgroup",
		"/sys/fs/cgroup/user.slice/session-1.scope", "/sys/fs/cgroup/user.slice/session-1.scope/cpu.stat")

	// Trailing slash in the relative path must be trimmed.
	f("0::/\n", "/sys/fs/cgroup", "/sys/fs/cgroup", "/sys/fs/cgroup/cpu.stat")

	// Hybrid mode: PSI from cgroup v2 at /sys/fs/cgroup/unified, cpu.stat from cgroup v1.
	// See https://github.com/VictoriaMetrics/metrics/issues/127
	f("4:cpu,cpuacct:/app.service\n0::/app.service\n", "/sys/fs/cgroup/unified",
		"/sys/fs/cgroup/unified/app.service", "/sys/fs/cgroup/cpu/app.service/cpu.stat")

	// Legacy cgroup v1 only.
	f("4:cpu,cpuacct:/app.service\n", "", "", "/sys/fs/cgroup/cpu/app.service/cpu.stat")

	// cgroup v2 entry present, but cgroup v2 isn't mounted.
	f("0::/user.slice\n", "", "", "")

	// Unified mode without cgroup v2 entry.
	f("1:name=systemd:/user.slice\n", "/sys/fs/cgroup", "", "")
}

func TestGetCgroupV1RelativePath(t *testing.T) {
	f := func(want, cgroupData string) {
		t.Helper()
		got := getCgroupV1RelativePath(cgroupData)
		if got != want {
			t.Fatalf("unexpected result: %q, want: %q", got, want)
		}
	}

	// cgroup v1 cpu controller.
	f("/daemons", "5:cpuacct,cpu,cpuset:/daemons\n")
	f("/", "5:cpuacct,cpu,cpuset:/\n")

	// Hybrid mode: both cgroup v1 cpu controller and cgroup v2 entries.
	f("/system.slice/app.service",
		"4:cpu,cpuacct:/system.slice/app.service\n"+
			"0::/system.slice/app.service\n")

	// No cpu controller in cgroup v1.
	f("", "5:memory:/daemons\n")

	// Pure cgroup v2.
	f("", "0::/user.slice\n")
}

func TestParseCgroupCPUStat(t *testing.T) {
	f := func(data string, want uint64) {
		t.Helper()
		ccs, err := parseCgroupCPUStat(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ccs.throttledNanosecs != want {
			t.Fatalf("unexpected throttledNanosecs: %d, want: %d", ccs.throttledNanosecs, want)
		}
	}

	// cgroup v2
	f("usage_usec 123\nuser_usec 45\nsystem_usec 67\nnr_periods 7\nnr_throttled 3\nthrottled_usec 1000000\n", 1e9)

	// cgroup v1
	f("nr_periods 9\nnr_throttled 4\nthrottled_time 2000000000\n", 2e9)

	// without throttling field
	f("usage_usec 123\nuser_usec 45\nsystem_usec 67\n", 0)

	t.Run("bad-value", func(t *testing.T) {
		_, err := parseCgroupCPUStat("throttled_usec nope\n")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
