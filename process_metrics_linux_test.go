package metrics

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

var testdir string

func init() {
	testdir, _ = os.Getwd()
	testdir += "/testdata/"
}

func getTestData(filename string, t *testing.T) string {
	data, err := os.ReadFile(testdir + filename)
	if err != nil {
		t.Fatalf("%v", err)
	}
	s := string(data)
	if filename == "linux.proc_metrics.out" {
		// since linux stat.starttime is relative to boot, we need to adjust
		// the expected results regarding this.
		m := regexp.MustCompile("process_start_time_seconds [0-9]+")
		n := fmt.Sprintf("process_start_time_seconds %d", startTimeSeconds)
		return m.ReplaceAllString(s, n)
	}
	return s
}

func stripComments(input string) string {
	var builder strings.Builder
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "#") || s == "" {
			continue
		}
		builder.WriteString(line + "\n")
	}
	return builder.String()
}

func Test_processMetrics(t *testing.T) {
	diffFormat := "Test %s:\n\tgot:\n'%v'\n\twant:\n'%v'"
	tests := []struct {
		name  string
		wantW string
		fn    func(w io.Writer)
	}{
		{"pm", getTestData("linux.proc_metrics.out", t), writeProcessMetrics},
		{"fdm", getTestData("linux.fd_metrics.out", t), writeFDMetrics},
	}
	for _, compact := range []bool{true, false} {
		ExposeMetadata(!compact)
		for _, tt := range tests {
			want := tt.wantW
			if compact {
				want = stripComments(want)
			}
			t.Run(tt.name, func(t *testing.T) {
				w := &bytes.Buffer{}
				tt.fn(w)
				if gotW := w.String(); gotW != want {
					t.Errorf(diffFormat, tt.name, gotW, want)
				}
			})
		}
	}

	// missing /proc/<pid>/io file - just omit the process_io_* metric entries
	// see https://github.com/VictoriaMetrics/metrics/issues/42
	tt := tests[0]
	want := stripComments(tt.wantW)
	m := regexp.MustCompile("process_io_[_a-z]+ [0-9]+\n")
	wantW := m.ReplaceAllString(want, "")
	testfiles[FD_IO] = "/doesNotExist"
	ExposeMetadata(false) // no need to check comments again
	init2()
	t.Run(tt.name, func(t *testing.T) {
		w := &bytes.Buffer{}
		tt.fn(w)
		if gotW := w.String(); gotW != wantW {
			t.Errorf(diffFormat, tt.name, gotW, wantW)
		}
	})

	// bad limits: just omit the process_max_fds metric entry
	tt = tests[1]
	want = stripComments(tt.wantW)
	m = regexp.MustCompile("process_max_fds [0-9]+\n")
	wantW = m.ReplaceAllString(want, "")
	testfiles[fdLimits] = "/limits_bad"
	init2()
	t.Run(tt.name, func(t *testing.T) {
		w := &bytes.Buffer{}
		tt.fn(w)
		if gotW := w.String(); gotW != wantW {
			t.Errorf(diffFormat, tt.name, gotW, wantW)
		}
	})

}
