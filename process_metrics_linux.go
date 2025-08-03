package metrics

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// See https://github.com/prometheus/procfs/blob/a4ac0826abceb44c40fc71daed2b301db498b93e/proc_stat.go#L40 .
const userHZ = 100

// Different environments may have different page size.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6457
var pageSizeBytes = uint64(os.Getpagesize())

// See http://man7.org/linux/man-pages/man5/proc.5.html
type procStat struct {
	State       byte
	Ppid        int
	Pgrp        int
	Session     int
	TtyNr       int
	Tpgid       int
	Flags       uint
	Minflt      uint
	Cminflt     uint
	Majflt      uint
	Cmajflt     uint
	Utime       uint
	Stime       uint
	Cutime      int
	Cstime      int
	Priority    int
	Nice        int
	NumThreads  int
	ItrealValue int
	Starttime   uint64
	Vsize       uint
	Rss         int
}

type ProcFd uint32

const (
	FD_LIMITS ProcFd = iota
	FD_STAT
	FD_IO
	FD_MEM
	FD_COUNT
)

// Testfiles in the same order as above.
var testfiles = [FD_COUNT]string{
	"/linux.ps_limits",
	"/linux.ps_stat",
	"/linux.ps_io",
	"/linux.ps_status",
}

/*
process metrics related file descriptors for files we always need, and

	do not want to open/close all the time
*/
var pm_fd [FD_COUNT]int

/*
to avoid, that go closes the files in the background, which makes the FDs

	above useless, we need to keep the reference to them as well
*/
var pm_file [FD_COUNT]*os.File

/* path used to count open FDs */
var fd_path string

/* path to get fd limits */
var limits_path string

/* Max open files soft limit for this process */
var maxOpenFDs float64 = 0

var STAT_START = 0
var NO_OUTPUT = false

func init2() {
	var testdata_dir = ""
	var onTest = len(os.Args) > 1 && strings.HasSuffix(os.Args[0], ".test")
	if onTest {
		cwd, err := os.Getwd()
		if err != nil {
			panic("Unknwon current working directory: " + err.Error())
		}
		testdata_dir = cwd + "/testdata"
		fmt.Printf("Using test data in %s ...\n", testdata_dir)
	}
	for i := 0; i < int(FD_COUNT); i++ {
		pm_fd[i] = -1
	}
	if onTest {
		fd_path = testdata_dir + "/fd"
		limits_path = testdata_dir + testfiles[FD_LIMITS]
	} else {
		fd_path = "/proc/self/fd"
		limits_path = "/proc/self/limits"
	}
	maxOpenFDs = float64(getMaxFilesLimit())

	// files to keep open
	var path string
	if onTest {
		path = testdata_dir + testfiles[FD_STAT]
	} else {
		path = "/proc/self/stat"
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		log.Printf("WARN: Unable to open %s (%v).", path, err)
	} else {
		// pid and "comm" field do not change over this process lifetime, so lets
		// precompute the number of bytes that can always be skipped (max 8+17+2).
		var data [32]byte
		pm_file[FD_STAT] = f
		pm_fd[FD_STAT] = int(f.Fd())
		n, err := syscall.Pread(pm_fd[FD_STAT],
			(*(*[unsafe.Sizeof(data) - 1]byte)(unsafe.Pointer(&data)))[:], 0)
		if err != nil {
			log.Printf("WARN: %s read error (%s).", path, err)
			pm_fd[FD_STAT] = -1
			f.Close()
		} else {
			for i := 0; i < n; i++ {
				// lookup the ') ' suffix for the 2nd field. If someone renames it
				// to something stupid, it does not deserve getting stats ;-)
				if data[i] == 0x29 && data[i+1] == 0x20 {
					STAT_START = i + 2
					break
				}
			}
			if STAT_START == 0 {
				pm_fd[FD_STAT] = -1 // should never happen
				f.Close()
			}
		}
	}

	if onTest {
		path = testdata_dir + testfiles[FD_IO]
	} else {
		path = "/proc/self/io"
	}
	f, err = os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		log.Printf("WARN: Unable to open %s (%v).", path, err)
	} else {
		pm_file[FD_IO] = f
		pm_fd[FD_IO] = int(f.Fd())
	}

	if onTest {
		path = testdata_dir + testfiles[FD_MEM]
	} else {
		path = "/proc/self/status"
	}
	f, err = os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		log.Printf("WARN: Unable to open %s (%v).", path, err)
	} else {
		pm_file[FD_MEM] = f
		pm_fd[FD_MEM] = int(f.Fd())
	}
}

func init() {
	init2()
}

func writeProcessMetrics(w io.Writer) {
	writeProcessMemMetrics(w)
	writeIOMetrics(w)
	var data [512]byte
	if pm_fd[FD_STAT] < 0 {
		return
	}
	n, err := syscall.Pread(pm_fd[FD_STAT],
		(*(*[unsafe.Sizeof(data) - 1]byte)(unsafe.Pointer(&data)))[:], 0)
	if err != nil {
		log.Printf("WARN: %s read error (%s).", pm_file[FD_STAT].Name(), err)
		return
	}
	data[n] = 0
	var p procStat
	_, err = fmt.Fscanf(bytes.NewReader(data[STAT_START:n]),
		"%c %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
		&p.State, &p.Ppid, &p.Pgrp, &p.Session, &p.TtyNr, &p.Tpgid, &p.Flags, &p.Minflt, &p.Cminflt, &p.Majflt, &p.Cmajflt,
		&p.Utime, &p.Stime, &p.Cutime, &p.Cstime, &p.Priority, &p.Nice, &p.NumThreads, &p.ItrealValue, &p.Starttime, &p.Vsize, &p.Rss)
	if err != nil {
		log.Printf("WARN: %s parse error in '%q' (%s).", pm_file[FD_STAT].Name(), data, err)
		return
	}

	// It is expensive obtaining `process_open_fds` when big number of file descriptors is opened,
	// so don't do it here.
	// See writeFDMetrics instead.

	utime := float64(p.Utime) / userHZ
	stime := float64(p.Stime) / userHZ
	WriteCounterFloat64(w, "process_cpu_seconds_system_total", stime)
	WriteCounterFloat64(w, "process_cpu_seconds_total", utime+stime)
	WriteCounterFloat64(w, "process_cpu_seconds_user_total", utime)
	WriteCounterUint64(w, "process_major_pagefaults_total", uint64(p.Majflt))
	WriteCounterUint64(w, "process_minor_pagefaults_total", uint64(p.Minflt))
	WriteGaugeUint64(w, "process_num_threads", uint64(p.NumThreads))
	WriteGaugeUint64(w, "process_resident_memory_bytes", uint64(p.Rss)*pageSizeBytes)
	WriteGaugeUint64(w, "process_start_time_seconds", uint64(startTimeSeconds))
	WriteGaugeUint64(w, "process_virtual_memory_bytes", uint64(p.Vsize))
}

func writeIOMetrics(w io.Writer) {
	var data [256]byte // 83 + 7*20 = 223
	if pm_fd[FD_IO] < 0 {
		return
	}
	n, err := syscall.Pread(pm_fd[FD_IO],
		(*(*[unsafe.Sizeof(data) - 1]byte)(unsafe.Pointer(&data)))[:], 0)
	if err != nil {
		log.Printf("WARN: %s read error (%s)", pm_file[FD_IO].Name(), err)
		return
	}
	data[n] = 0
	getInt := func(s string) int64 {
		n := strings.IndexByte(s, ' ')
		if n < 0 {
			log.Printf("WARN: %s no whitespace in '%q'.", pm_file[FD_IO].Name(), s)
			return 0
		}
		v, err := strconv.ParseInt(s[n+1:], 10, 64)
		if err != nil {
			log.Printf("WARN: %s parse error in '%q' (%s)", pm_file[FD_IO].Name(), s, err)
			return 0
		}
		return v
	}
	var rchar, wchar, syscr, syscw, readBytes, writeBytes int64
	lines := strings.Split(string(data[:n]), "\n")
	for _, s := range lines {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "rchar: "):
			rchar = getInt(s)
		case strings.HasPrefix(s, "wchar: "):
			wchar = getInt(s)
		case strings.HasPrefix(s, "syscr: "):
			syscr = getInt(s)
		case strings.HasPrefix(s, "syscw: "):
			syscw = getInt(s)
		case strings.HasPrefix(s, "read_bytes: "):
			readBytes = getInt(s)
		case strings.HasPrefix(s, "write_bytes: "):
			writeBytes = getInt(s)
		}
	}
	WriteGaugeUint64(w, "process_io_read_bytes_total", uint64(rchar))
	WriteGaugeUint64(w, "process_io_written_bytes_total", uint64(wchar))
	WriteGaugeUint64(w, "process_io_read_syscalls_total", uint64(syscr))
	WriteGaugeUint64(w, "process_io_write_syscalls_total", uint64(syscw))
	WriteGaugeUint64(w, "process_io_storage_read_bytes_total", uint64(readBytes))
	WriteGaugeUint64(w, "process_io_storage_written_bytes_total", uint64(writeBytes))
}

// In Linux the startime shown in /proc/<pid>/stat field 22 is in ticks since
// boot and thus the exact starttime since epoch in seconds would be:
//
//	Now() - $(</proc/uptime) + stat.starttime/ticksPerSecond
//
// However, since "global" vars get evaluated at app start, now() should be good
// enough.
var startTimeSeconds = time.Now().Unix()

// writeFDMetrics writes process_max_fds and process_open_fds metrics to w.
func writeFDMetrics(w io.Writer) {
	if maxOpenFDs != 0 {
		WriteGaugeFloat64(w, "process_max_fds", maxOpenFDs)
	}
	totalOpenFDs := getOpenFDsCount()
	if totalOpenFDs > 0 {
		WriteGaugeUint64(w, "process_open_fds", uint64(totalOpenFDs))
	}
}

/** return 0 on error, the number of open files otherwise */
func getOpenFDsCount() int32 {
	f, err := os.Open(fd_path)
	if err != nil {
		return 0
	}
	defer f.Close()
	var totalOpenFDs = 0
	for {
		names, err := f.Readdirnames(512)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("WARN: %s read error (%s)", fd_path, err)
		} else {
			totalOpenFDs += len(names)
		}
	}
	return int32(totalOpenFDs)
}

/* returns 0 on error, -1 for unlimited, the limit otherwise */
func getMaxFilesLimit() int32 {
	data, err := os.ReadFile(limits_path)
	if err != nil {
		return 0
	}
	lines := strings.Split(string(data), "\n")
	const prefix = "Max open files"
	for _, s := range lines {
		if !strings.HasPrefix(s, prefix) {
			continue
		}
		text := strings.TrimSpace(s[len(prefix):])
		// Extract soft limit.
		n := strings.IndexByte(text, ' ')
		if n < 0 {
			log.Printf("WARN: %s no soft limit found in '%q'", limits_path, s)
			return 0
		}
		text = text[:n]
		if text == "unlimited" {
			return -1
		}
		limit, err := strconv.ParseInt(text, 10, 64)
		if err != nil || limit < 0 || limit > math.MaxInt32 {
			log.Printf("WARN: %s no valid soft limit in '%q' (%s).", limits_path, s, err)
			return 0
		}
		return int32(limit)
	}
	log.Printf("WARN: %s no max open files limit found", limits_path)
	return 0
}

// https://man7.org/linux/man-pages/man5/procfs.5.html
type memStats struct {
	vmPeak   uint64
	rssPeak  uint64
	rssAnon  uint64
	rssFile  uint64
	rssShmem uint64
}

func writeProcessMemMetrics(w io.Writer) {
	ms := getMemStats()
	if ms == nil {
		return
	}
	WriteGaugeUint64(w, "process_virtual_memory_peak_bytes", ms.vmPeak)
	WriteGaugeUint64(w, "process_resident_memory_peak_bytes", ms.rssPeak)
	WriteGaugeUint64(w, "process_resident_memory_anon_bytes", ms.rssAnon)
	WriteGaugeUint64(w, "process_resident_memory_file_bytes", ms.rssFile)
	WriteGaugeUint64(w, "process_resident_memory_shared_bytes", ms.rssShmem)

}

func getMemStats() *memStats {
	var data [2048]byte // 571 + 2*57 + 57*20 = 1825  so 2048 should be safe
	if pm_fd[FD_MEM] < 0 {
		return nil
	}
	n, err := syscall.Pread(pm_fd[FD_MEM],
		(*(*[unsafe.Sizeof(data) - 1]byte)(unsafe.Pointer(&data)))[:], 0)
	if err != nil {
		log.Printf("WARN: %s read error (%s).", pm_file[FD_MEM].Name(), err)
		return nil
	}
	data[n] = 0
	var ms memStats
	lines := strings.Split(string(data[:n]), "\n")
	for _, s := range lines {
		if !strings.HasPrefix(s, "Vm") && !strings.HasPrefix(s, "Rss") {
			continue
		}
		// Extract key value.
		line := strings.Fields(s)
		if len(line) != 3 {
			log.Printf("WARN: %s unexpected number of fields in '%q' (%d != %d).",
				pm_file[FD_MEM].Name(), s, len(line), 3)
			return nil
		}
		memStatName := line[0]
		memStatValue := line[1]
		value, err := strconv.ParseUint(memStatValue, 10, 64)
		if err != nil {
			log.Printf("WARN: %s  number parse error in '%q' (%s)", pm_file[FD_MEM].Name(), s, err)
			return nil
		}
		if line[2] != "kB" {
			log.Printf("WARN: %s expecting kB value in '%q' (got '%q')", pm_file[FD_MEM].Name(), s, line[2])
			return nil
		}
		value <<= 10
		switch memStatName {
		case "VmPeak:":
			ms.vmPeak = value
		case "VmHWM:":
			ms.rssPeak = value
		case "RssAnon:":
			ms.rssAnon = value
		case "RssFile:":
			ms.rssFile = value
		case "RssShmem:":
			ms.rssShmem = value
		}
	}
	return &ms
}
