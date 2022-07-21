package metrics

import (
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
	f := func(interval time.Duration, extraLabels string) {
		t.Helper()
		defer func() {
			if err := recover(); err == nil {
				panic("expecting non-nil error")
			}
		}()
		InitPush("http://foobar", interval, extraLabels, false)
	}

	// Non-positive interval
	f(0, "")

	// Invalid extraLabels
	f(time.Second, "foo")
	f(time.Second, "foo{bar")
	f(time.Second, "foo=bar")
	f(time.Second, "foo='bar'")
	f(time.Second, `foo="bar",baz`)
	f(time.Second, `{foo="bar"}`)
	f(time.Second, `a{foo="bar"}`)
}
