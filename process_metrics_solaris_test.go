/* go test -timeout 30s -run '^Test_write*' */

// Author: Jens Elkner (C) 2025

package metrics

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

var testdir string

func init() {
	testdir, _ = os.Getwd()
	testdir += "/testdata/"
}

func getTestData(filename string) string {
	data, err := os.ReadFile(testdir + filename)
	if err != nil {
		fmt.Printf("Error %v\n", err)
		return ""
	}
	return string(data)
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
	tests := []struct {
		name  string
		wantW string
		fn    func(w io.Writer)
	}{
		{"pm", getTestData("solaris.proc_metrics.out"), writeProcessMetrics},
		{"fdm", getTestData("solaris.fd_metrics.out"), writeFDMetrics},
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
					t.Errorf("Test %s:\n\tgot:\n'%v'\n\twant:\n'%v'", tt.name, gotW, want)
				}
			})
		}
	}
}
