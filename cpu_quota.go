package metrics

import (
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

func availableCPUCores() float64 {
	fallback := float64(runtime.NumCPU())
	if runtime.GOOS != "linux" {
		return fallback
	}
	membership, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return fallback
	}
	return availableCPUCoresFromCgroup(string(membership), cpuMountInfo(), os.ReadFile, fallback)
}

// Cache mountinfo, whose generation can block on an unresponsive mount. Only
// one read is attempted, so a stuck mount cannot leak a goroutine on each scrape.
var cpuMountInfo = sync.OnceValue(func() string {
	ch := make(chan string, 1)
	go func() {
		data, _ := os.ReadFile("/proc/self/mountinfo")
		ch <- string(data)
	}()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case data := <-ch:
		return data
	case <-timer.C:
		return ""
	}
})

// Use the smallest positive quota across the process's visible cgroup ancestors.
// Quotas are shared upper bounds, not a reservation or a GOMAXPROCS setting.
func availableCPUCoresFromCgroup(membership, mountinfo string, readFile func(string) ([]byte, error), fallback float64) float64 {
	var quota float64
	for line := range strings.SplitSeq(mountinfo, "\n") {
		before, after, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		mount := strings.Fields(before)
		fs := strings.Fields(after)
		if len(mount) < 5 || len(fs) < 3 {
			continue
		}
		v2 := fs[0] == "cgroup2"
		if !v2 && (fs[0] != "cgroup" || !hasCPUController(fs[2])) {
			continue
		}
		root := unescapeCPUMountPath(mount[3])
		mountpoint := unescapeCPUMountPath(mount[4])
		if !path.IsAbs(root) || !path.IsAbs(mountpoint) {
			continue
		}
		root, mountpoint = path.Clean(root), path.Clean(mountpoint)
		group := cpuCgroupPath(membership, v2)
		if group == "" {
			continue
		}
		var relative string
		if group != root {
			var underRoot bool
			relative, underRoot = strings.CutPrefix(group, strings.TrimSuffix(root, "/")+"/")
			if !underRoot {
				continue
			}
		}
		for dir := path.Join(mountpoint, relative); ; dir = path.Dir(dir) {
			q := readCPUQuota(dir, v2, readFile)
			if q > 0 && (quota == 0 || q < quota) {
				quota = q
			}
			if dir == mountpoint {
				break
			}
		}
	}
	if quota > 0 {
		return quota
	}
	return fallback
}

func cpuCgroupPath(membership string, v2 bool) string {
	for line := range strings.SplitSeq(membership, "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 || !path.IsAbs(fields[2]) {
			continue
		}
		if (v2 && fields[0] == "0" && fields[1] == "") || (!v2 && hasCPUController(fields[1])) {
			return path.Clean(fields[2])
		}
	}
	return ""
}

func hasCPUController(controllers string) bool {
	return strings.Contains(","+controllers+",", ",cpu,")
}

func unescapeCPUMountPath(s string) string {
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(s)
}

func readCPUQuota(dir string, v2 bool, readFile func(string) ([]byte, error)) float64 {
	var fields []string
	if v2 {
		data, err := readFile(path.Join(dir, "cpu.max"))
		if err != nil {
			return 0
		}
		fields = strings.Fields(string(data))
	} else {
		quota, err := readFile(path.Join(dir, "cpu.cfs_quota_us"))
		if err != nil {
			return 0
		}
		period, err := readFile(path.Join(dir, "cpu.cfs_period_us"))
		if err != nil {
			return 0
		}
		fields = []string{strings.TrimSpace(string(quota)), strings.TrimSpace(string(period))}
	}
	if len(fields) != 2 {
		return 0
	}
	quota, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil || quota == 0 {
		return 0
	}
	period, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil || period == 0 {
		return 0
	}
	return float64(quota) / float64(period)
}
