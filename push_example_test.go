package metrics_test

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

func ExampleInitPushWithConfig() {
	syncCh := make(chan string)
	srv := newServer(syncCh)
	defer srv.Close()

	cfg := metrics.PushConfig{
		URL:      srv.URL,
		Interval: time.Millisecond * 100,
		WriteMetricsFn: func(w io.Writer) {
			fmt.Fprint(w, "foo{label=\"bar\"} 1\n")
			fmt.Fprint(w, "foo{label=\"baz\"} 2\n")
		},
	}
	if err := metrics.InitPushWithConfig(cfg); err != nil {
		panic(fmt.Sprintf("BUG: unexpected error: %s", err))
	}
	fmt.Println(<-syncCh)

	// Output:
	// foo{label="bar"} 1
	// foo{label="baz"} 2
}

func ExampleInitPushExt() {
	syncCh := make(chan string)
	srv := newServer(syncCh)
	defer srv.Close()

	writeFn := func(w io.Writer) {
		fmt.Fprint(w, "foo{label=\"bar\"} 11\n")
		fmt.Fprint(w, "foo{label=\"baz\"} 22\n")
	}

	err := metrics.InitPushExt(srv.URL, time.Millisecond*100, "", writeFn)
	if err != nil {
		panic(fmt.Sprintf("BUG: unexpected error: %s", err))
	}
	fmt.Println(<-syncCh)

	// Output:
	// foo{label="bar"} 11
	// foo{label="baz"} 22
}

func newServer(ch chan string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			panic(fmt.Sprintf("BUG: unexpected error: %s", err))
		}
		defer gr.Close()

		b, err := io.ReadAll(gr)
		if err != nil {
			panic(fmt.Sprintf("BUG: unexpected error: %s", err))
		}
		ch <- string(b)
	}))
	return srv
}
