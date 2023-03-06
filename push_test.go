package metrics

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestAddExtraLabels(t *testing.T) {
	f := func(s, extraLabels, expectedResult string) {
		t.Helper()
		result := addExtraLabels(nil, []byte(s), extraLabels)
		if string(result) != expectedResult {
			t.Fatalf("unexpected result; got\n%s\nwant\n%s", result, expectedResult)
		}
	}
	f("", `foo="bar"`, "")
	f("a 123", `foo="bar"`, `a{foo="bar"} 123`+"\n")
	f(`a{b="c"} 1.3`, `foo="bar"`, `a{foo="bar",b="c"} 1.3`+"\n")
	f(`a{b="c}{"} 1.3`, `foo="bar",baz="x"`, `a{foo="bar",baz="x",b="c}{"} 1.3`+"\n")
	f(`foo 1
bar{a="x"} 2
`, `foo="bar"`, `foo{foo="bar"} 1
bar{foo="bar",a="x"} 2
`)
	f(`
foo 1
# some counter
# type foobar counter
	  foobar{a="b",c="d"} 4`, `x="y"`, `foo{x="y"} 1
# some counter
# type foobar counter
foobar{x="y",a="b",c="d"} 4
`)
}

func TestInitPushFailure(t *testing.T) {
	f := func(pushURL string, interval time.Duration, extraLabels string) {
		t.Helper()
		if err := InitPush(pushURL, interval, extraLabels, false); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// Invalid url
	f("foobar", time.Second, "")
	f("aaa://foobar", time.Second, "")
	f("http:///bar", time.Second, "")

	// Non-positive interval
	f("http://foobar", 0, "")
	f("http://foobar", -time.Second, "")

	// Invalid extraLabels
	f("http://foobar", time.Second, "foo")
	f("http://foobar", time.Second, "foo{bar")
	f("http://foobar", time.Second, "foo=bar")
	f("http://foobar", time.Second, "foo='bar'")
	f("http://foobar", time.Second, `foo="bar",baz`)
	f("http://foobar", time.Second, `{foo="bar"}`)
	f("http://foobar", time.Second, `a{foo="bar"}`)
}

func TestInitPushURLsExt(t *testing.T) {

	tests := []struct {
		name           string
		pushURLs       []string
		interval       time.Duration
		extraLabels    string
		expectedMetric []byte
		sendMetric     string
		writeMetrics   func(w io.Writer)
		count          uint64
		wantErr        bool
	}{
		{
			name:           "empty URLs",
			pushURLs:       nil,
			interval:       time.Second * 1,
			extraLabels:    "",
			expectedMetric: nil,
			sendMetric:     "",
			writeMetrics:   func(w io.Writer) {},
			count:          0,
			wantErr:        false,
		},
		{
			name:           "wrong URL",
			pushURLs:       []string{"http:/123"},
			interval:       time.Second * 1,
			extraLabels:    "",
			expectedMetric: nil,
			sendMetric:     "",
			writeMetrics:   func(w io.Writer) {},
			count:          0,
			wantErr:        true,
		},
		{
			name:           "negative duration interval",
			pushURLs:       []string{"http:/123"},
			interval:       time.Second * -1,
			extraLabels:    "",
			expectedMetric: nil,
			sendMetric:     "",
			writeMetrics:   func(w io.Writer) {},
			count:          0,
			wantErr:        true,
		},
		{
			name:           "incorrect extra labels",
			pushURLs:       []string{"http://123"},
			interval:       time.Millisecond * -1,
			extraLabels:    "123123,123123",
			expectedMetric: nil,
			sendMetric:     "",
			writeMetrics:   func(w io.Writer) {},
			count:          0,
			wantErr:        true,
		},
		{
			name:           "correct all income params",
			pushURLs:       []string{},
			interval:       time.Millisecond * 500,
			extraLabels:    "foo=\"bar\"",
			sendMetric:     `vm_app_version{version="123", short_version="123"} 1`,
			expectedMetric: []byte("vm_app_version{foo=\"bar\",version=\"123\", short_version=\"123\"} 1\n"),
			count:          5,
			writeMetrics:   func(w io.Writer) {},
			wantErr:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var count uint64
			atomic.StoreUint64(&count, 0)
			done := make(chan struct{})

			s := httptest.NewServer(getReadHandler(t, tt.expectedMetric, count, func(count uint64) {
				if tt.count <= count {
					done <- struct{}{}
				}
			}))
			defer s.Close()

			if tt.sendMetric != "" {
				tt.writeMetrics = func(w io.Writer) { fmt.Fprintf(w, tt.sendMetric) }
				tt.pushURLs = append(tt.pushURLs, s.URL)
			}

			if err := InitPushByURLsExt(tt.pushURLs, tt.interval, tt.extraLabels, tt.writeMetrics); (err != nil) != tt.wantErr {
				t.Errorf("InitPushURLsExt() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(tt.sendMetric) == 0 {
				close(done)
			}
			<-done
		})
	}
}

func getReadHandler(t *testing.T, expected []byte, count uint64, f func(count uint64)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&count, 1)
		zw, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatal(err)
			return
		}

		defer func() {
			r.Body.Close()
			zw.Close()

			val := atomic.LoadUint64(&count)
			f(val)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Ok"))
		}()

		got, err := io.ReadAll(zw)
		if err != nil {
			t.Fatal(err)
			return
		}

		if bytes.Compare(got, expected) != 0 {
			t.Fatalf("got metric: %s; expected: %s", got, expected)
			return
		}
	})
}
