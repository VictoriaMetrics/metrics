package metrics

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestInitPushWithOptions(t *testing.T) {
	f := func(s *Set, opts *PushOptions, expectedHeaders, expectedData string) {
		t.Helper()

		var reqHeaders []byte
		var reqData []byte
		var reqErr error
		doneCh := make(chan struct{})
		firstRequest := true
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if firstRequest {
				var bb bytes.Buffer
				r.Header.WriteSubset(&bb, map[string]bool{
					"Accept-Encoding": true,
					"Content-Length":  true,
					"User-Agent":      true,
				})
				reqHeaders = bb.Bytes()
				reqData, reqErr = io.ReadAll(r.Body)
				close(doneCh)
				firstRequest = false
			}
		}))
		defer srv.Close()
		ctx, cancel := context.WithCancel(context.Background())
		if err := s.InitPushWithOptions(ctx, srv.URL, time.Millisecond, opts); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		select {
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout!")
		case <-doneCh:
			// stop the periodic pusher
			cancel()
		}
		if reqErr != nil {
			t.Fatalf("unexpected error: %s", reqErr)
		}
		if opts == nil || !opts.DisableCompression {
			zr, err := gzip.NewReader(bytes.NewBuffer(reqData))
			if err != nil {
				t.Fatalf("cannot initialize gzip reader: %s", err)
			}
			data, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("cannot read data from gzip reader: %s", err)
			}
			if err := zr.Close(); err != nil {
				t.Fatalf("unexpected error when closing gzip reader: %s", err)
			}
			reqData = data
		}
		if string(reqHeaders) != expectedHeaders {
			t.Fatalf("unexpected request headers; got\n%s\nwant\n%s", reqHeaders, expectedHeaders)
		}
		if string(reqData) != expectedData {
			t.Fatalf("unexpected data; got\n%s\nwant\n%s", reqData, expectedData)
		}
	}

	s := NewSet()
	c := s.NewCounter("foo")
	c.Set(1234)
	_ = s.NewGauge("bar", func() float64 {
		return 42.12
	})

	// nil PushOptions
	f(s, nil, "Content-Encoding: gzip\r\nContent-Type: text/plain\r\n", "bar 42.12\nfoo 1234\n")

	// Disable compression on the pushed request body
	f(s, &PushOptions{
		DisableCompression: true,
	}, "Content-Type: text/plain\r\n", "bar 42.12\nfoo 1234\n")

	// Add extra labels
	f(s, &PushOptions{
		ExtraLabels: `label1="value1",label2="value2"`,
	}, "Content-Encoding: gzip\r\nContent-Type: text/plain\r\n", `bar{label1="value1",label2="value2"} 42.12`+"\n"+`foo{label1="value1",label2="value2"} 1234`+"\n")

	// Add extra headers
	f(s, &PushOptions{
		Headers: []string{"Foo: Bar", "baz:aaaa-bbb"},
	}, "Baz: aaaa-bbb\r\nContent-Encoding: gzip\r\nContent-Type: text/plain\r\nFoo: Bar\r\n", "bar 42.12\nfoo 1234\n")
}

func TestPushMetrics(t *testing.T) {
	f := func(s *Set, opts *PushOptions, expectedHeaders, expectedData string) {
		t.Helper()

		var reqHeaders []byte
		var reqData []byte
		var reqErr error
		doneCh := make(chan struct{})
		firstRequest := true
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if firstRequest {
				var bb bytes.Buffer
				r.Header.WriteSubset(&bb, map[string]bool{
					"Accept-Encoding": true,
					"Content-Length":  true,
					"User-Agent":      true,
				})
				reqHeaders = bb.Bytes()
				reqData, reqErr = io.ReadAll(r.Body)
				close(doneCh)
				firstRequest = false
			}
		}))
		defer srv.Close()
		ctx := context.Background()
		if err := s.PushMetrics(ctx, srv.URL, opts); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		select {
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout!")
		case <-doneCh:
		}
		if reqErr != nil {
			t.Fatalf("unexpected error: %s", reqErr)
		}
		if opts == nil || !opts.DisableCompression {
			zr, err := gzip.NewReader(bytes.NewBuffer(reqData))
			if err != nil {
				t.Fatalf("cannot initialize gzip reader: %s", err)
			}
			data, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("cannot read data from gzip reader: %s", err)
			}
			if err := zr.Close(); err != nil {
				t.Fatalf("unexpected error when closing gzip reader: %s", err)
			}
			reqData = data
		}
		if string(reqHeaders) != expectedHeaders {
			t.Fatalf("unexpected request headers; got\n%s\nwant\n%s", reqHeaders, expectedHeaders)
		}
		if string(reqData) != expectedData {
			t.Fatalf("unexpected data; got\n%s\nwant\n%s", reqData, expectedData)
		}
	}

	s := NewSet()
	c := s.NewCounter("foo")
	c.Set(1234)
	_ = s.NewGauge("bar", func() float64 {
		return 42.12
	})

	// nil PushOptions
	f(s, nil, "Content-Encoding: gzip\r\nContent-Type: text/plain\r\n", "bar 42.12\nfoo 1234\n")

	// Disable compression on the pushed request body
	f(s, &PushOptions{
		DisableCompression: true,
	}, "Content-Type: text/plain\r\n", "bar 42.12\nfoo 1234\n")

	// Add extra labels
	f(s, &PushOptions{
		ExtraLabels: `label1="value1",label2="value2"`,
	}, "Content-Encoding: gzip\r\nContent-Type: text/plain\r\n", `bar{label1="value1",label2="value2"} 42.12`+"\n"+`foo{label1="value1",label2="value2"} 1234`+"\n")

	// Add extra headers
	f(s, &PushOptions{
		Headers: []string{"Foo: Bar", "baz:aaaa-bbb"},
	}, "Baz: aaaa-bbb\r\nContent-Encoding: gzip\r\nContent-Type: text/plain\r\nFoo: Bar\r\n", "bar 42.12\nfoo 1234\n")
}
