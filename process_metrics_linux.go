package metrics

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const statFilepath = "/proc/self/stat"

// See https://github.com/prometheus/procfs/blob/a4ac0826abceb44c40fc71daed2b301db498b93e/proc_stat.go#L40 .
const userHZ = 100

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

func writeProcessMetrics(w io.Writer) {
	data, err := ioutil.ReadFile(statFilepath)
	if err != nil {
		log.Printf("ERROR: cannot open %s: %s", statFilepath, err)
		return
	}
	// Search for the end of command.
	n := bytes.LastIndex(data, []byte(") "))
	if n < 0 {
		log.Printf("ERROR: cannot find command in parentheses in %q read from %s", data, statFilepath)
		return
	}
	data = data[n+2:]

	var p procStat
	bb := bytes.NewBuffer(data)
	_, err = fmt.Fscanf(bb, "%c %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
		&p.State, &p.Ppid, &p.Pgrp, &p.Session, &p.TtyNr, &p.Tpgid, &p.Flags, &p.Minflt, &p.Cminflt, &p.Majflt, &p.Cmajflt,
		&p.Utime, &p.Stime, &p.Cutime, &p.Cstime, &p.Priority, &p.Nice, &p.NumThreads, &p.ItrealValue, &p.Starttime, &p.Vsize, &p.Rss)
	if err != nil {
		log.Printf("ERROR: cannot parse %q read from %s: %s", data, statFilepath, err)
		return
	}

	// It is expensive obtaining `process_open_fds` when big number of file descriptors is opened,
	// don't do it here.

	utime := float64(p.Utime) / userHZ
	stime := float64(p.Stime) / userHZ
	fmt.Fprintf(w, "process_cpu_seconds_system_total %g\n", stime)
	fmt.Fprintf(w, "process_cpu_seconds_total %g\n", utime+stime)
	fmt.Fprintf(w, "process_cpu_seconds_user_total %g\n", utime)
	fmt.Fprintf(w, "process_major_pagefaults_total %d\n", p.Majflt)
	fmt.Fprintf(w, "process_minor_pagefaults_total %d\n", p.Minflt)
	fmt.Fprintf(w, "process_num_threads %d\n", p.NumThreads)
	fmt.Fprintf(w, "process_resident_memory_bytes %d\n", p.Rss*4096)
	fmt.Fprintf(w, "process_start_time_seconds %d\n", startTimeSeconds)
	fmt.Fprintf(w, "process_virtual_memory_bytes %d\n", p.Vsize)
}

var startTimeSeconds = time.Now().Unix()

const fdReadChunkSize = 512

func WriteFDMetrics(w io.Writer){
	totalOpenFDs, err := getOpenFDsCount("/proc/self/fd")
	if err != nil {
		log.Printf("%v",err)
		return
	}
	maxOpenFDs, err := getMaxFilesLimit("/proc/self/limits")
	if err != nil {
		log.Printf("%v",err)
		return
	}
	fmt.Fprintf(w,"process_max_fds %d\n",maxOpenFDs)
	fmt.Fprintf(w,"process_open_fds %d\n",totalOpenFDs)
}


func getOpenFDsCount(path string)(uint64,error){
	f, err := os.Open(path)
	if err != nil {
		return 0,fmt.Errorf("ERROR: cannot open process fd path: %q, err: %v",path,err)
	}
	defer f.Close()
	var totalOpenFDs uint64
	for {
		names, err := f.Readdirnames(fdReadChunkSize)
		if err == io.EOF{
			break
		}
		if err != nil {
			return 0, fmt.Errorf("ERROR: unexpected error at readdirnames: %v",err)
		}
		totalOpenFDs += uint64(len(names))
	}
	return totalOpenFDs, nil
}

var limitsRe = regexp.MustCompile(`(Max \w+\s{0,1}?\w*\s{0,1}\w*)\s{2,}(\w+)\s+(\w+)`)

func getMaxFilesLimit(path string)(uint64,error){
	f, err := os.Open(path)
	if err != nil {
		return 0,fmt.Errorf("ERROR: cannot open path: %q for max files limit, err: %w",path,err)
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	// skip first line
	scan.Scan()
	for scan.Scan(){
		text := scan.Text()
		if !strings.HasPrefix(text,"Max open files"){
			continue
		}
		items := limitsRe.FindStringSubmatch(text)
		if len(items) != 4 {
			return 0,fmt.Errorf("ERROR: unxpected fields num for limits file, want: %d, got: %d, line: %q",4,len(items),text)
		}
		// use soft limit.
		limit := items[2]
		if limit == "unlimited"{
			return 18446744073709551615,nil
		}
		limitUint, err := strconv.ParseUint(limit,10,64)
		if err != nil {
			return 0, fmt.Errorf("ERROR: cannot parse limit: %q as uint64: %w",limit,err)
		}
		return limitUint,nil
	}
	return 0, fmt.Errorf("ERROR: max open files limit wasn't found")
}
