package metrics

import (
	"testing"
	"time"
)

func TestPushConfigValidateError(t *testing.T) {
	f := func(config *PushConfig) {
		t.Helper()
		if err := config.Validate(); err == nil {
			t.Fatalf("expecting non-nil error when validating %v", config)
		}
	}

	f(&PushConfig{})
	f(&PushConfig{URL: "", Interval: time.Second})
	f(&PushConfig{URL: "https://localhost:8080", Interval: -1 * time.Second})
	f(&PushConfig{URL: "htt://localhost:8080", Interval: time.Second})
	f(&PushConfig{URL: "http://localhost:8080", Interval: time.Second, ExtraLabels: "a{} "})
}

func TestPushConfigValidateSuccess(t *testing.T) {
	f := func(config *PushConfig) {
		t.Helper()
		if err := config.Validate(); err != nil {
			t.Fatalf("expecting nil error when validating %v; err: %s", config, err)
		}
	}

	f(&PushConfig{URL: "http://localhost:8080", Interval: time.Second})
	f(&PushConfig{URL: "http://localhost:8080", Interval: time.Second, ExtraLabels: `foo="bar"`})
}
