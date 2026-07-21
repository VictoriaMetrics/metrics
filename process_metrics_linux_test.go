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

func TestGetCgroupV2PathInternal(t *testing.T) {
	f := func(want, cgroupData, mountinfoData string) {
		t.Helper()
		got := getCgroupPathInternal(cgroupVersionV2, cgroupData, mountinfoData)
		if got != want {
			t.Fatalf("unexpected result: %q, want: %q", got, want)
		}
	}

	// Pure cgroup v2: unified hierarchy mounted at /sys/fs/cgroup.
	f("/sys/fs/cgroup/user.slice/user-1000.slice/session-1.scope",
		"0::/user.slice/user-1000.slice/session-1.scope\n",
		"30 23 0:26 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup2 rw,nsdelegate\n")

	// Hybrid cgroup mode: cgroup v2 mounted at /sys/fs/cgroup/unified.
	// See https://github.com/VictoriaMetrics/metrics/issues/127
	f("/sys/fs/cgroup/unified/user.slice/user-2390.slice/session-c625.scope",
		"1:name=systemd:/user.slice/user-2390.slice/session-c625.scope\n"+
			"0::/user.slice/user-2390.slice/session-c625.scope\n",
		"30 23 0:26 / /sys/fs/cgroup tmpfs rw - tmpfs tmpfs rw\n"+
			"31 30 0:27 / /sys/fs/cgroup/unified rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup2 rw,nsdelegate\n"+
			"32 30 0:28 / /sys/fs/cgroup/cpu,cpuacct rw - cgroup cgroup rw,cpu,cpuacct\n")

	// Mountpoint with optional fields before the " - " separator.
	f("/sys/fs/cgroup/foo",
		"0::/foo\n",
		"30 23 0:26 / /sys/fs/cgroup rw shared:5 - cgroup2 cgroup2 rw\n")

	// Trailing slash in the relative path must be trimmed.
	f("/sys/fs/cgroup",
		"0::/\n",
		"30 23 0:26 / /sys/fs/cgroup rw - cgroup2 cgroup2 rw\n")

	// No cgroup v2 entry (cgroup v1 only) -> empty path.
	f("",
		"1:name=systemd:/user.slice\n",
		"30 23 0:26 / /sys/fs/cgroup/systemd rw - cgroup cgroup rw,name=systemd\n")

	// cgroup v2 present in /proc/self/cgroup but not mounted -> fallback to /sys path.
	f("/sys/fs/cgroup/user.slice",
		"0::/user.slice\n",
		"30 23 0:26 / /sys/fs/cgroup rw - cgroup cgroup rw,cpu\n")
}

func TestGetCgroupV1CpuControllerPathInternal(t *testing.T) {
	f := func(want, cgroupData, mountinfoData string) {
		t.Helper()
		got := getCgroupV1CpuControllerPathInternal(cgroupData, mountinfoData)
		if got != want {
			t.Fatalf("unexpected result: %q, want: %q", got, want)
		}
	}

	f("/sys/fs/cgroup/cpu/daemons",
		"5:cpuacct,cpu,cpuset:/daemons\n",
		"32 30 0:28 / /sys/fs/cgroup/cpu rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,cpu\n")

	f("/sys/fs/cgroup/cpu,cpuacct/daemons",
		"5:cpuacct,cpu,cpuset:/daemons\n",
		"32 30 0:28 / /sys/fs/cgroup/cpu,cpuacct rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,cpu,cpuacct\n")

	f("/sys/fs/cgroup/cpu",
		"5:cpuacct,cpu,cpuset:/\n",
		"32 30 0:28 / /sys/fs/cgroup/cpu rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,cpu\n")

	f("",
		"5:memory:/daemons\n",
		"32 30 0:28 / /sys/fs/cgroup/cpu rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,cpu\n")

	f("/sys/fs/cgroup/cpu/daemons",
		"5:cpuacct,cpu,cpuset:/daemons\n",
		"")
}

func TestParseCgroupCpuStat(t *testing.T) {
	t.Run("cgroup-v2", func(t *testing.T) {
		ctms, err := parseCgroupCpuStat("usage_usec 123\nuser_usec 45\nsystem_usec 67\nnr_periods 7\nnr_throttled 3\nthrottled_usec 1000000\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := *ctms, (cpuThrottleMetrics{throttledSecs: 1}); got != want {
			t.Fatalf("unexpected result: %#v, want: %#v", got, want)
		}
	})

	t.Run("cgroup-v1", func(t *testing.T) {
		ctms, err := parseCgroupCpuStat("nr_periods 9\nnr_throttled 4\nthrottled_time 2000000000\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := *ctms, (cpuThrottleMetrics{throttledSecs: 2}); got != want {
			t.Fatalf("unexpected result: %#v, want: %#v", got, want)
		}
	})

	t.Run("bad-value", func(t *testing.T) {
		_, err := parseCgroupCpuStat("throttled_usec nope\n")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
