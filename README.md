[![GoDoc](https://godoc.org/github.com/VictoriaMetrics/metrics?status.svg)](http://godoc.org/github.com/VictoriaMetrics/metrics)
[![Go Report](https://goreportcard.com/badge/github.com/VictoriaMetrics/metrics)](https://goreportcard.com/report/github.com/VictoriaMetrics/metrics)

# metrics - lightweight package for exporting metrics in Prometheus format


### Features

* Lightweight. Has minimal number of third-party dependencies and all these deps are small.
  See [this article](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d) for details.
* Easy to use. See the [API docs](http://godoc.org/github.com/VictoriaMetrics/metrics).
* Fast.


### Limitations

* It doesn't implement advanced functionality from [github.com/prometheus/client_golang](https://godoc.org/github.com/prometheus/client_golang).


### Usage

```go
import "github.com/VictoriaMetrics/metrics"

// Register various time series.
// Time series name may contain labels in Prometheus format - see below.
var (
	// Register counter without labels.
	requestsTotal = metrics.NewCounter("requests_total")

	// Register summary with a single label.
	requestDuration = metrics.NewSummary(`requests_duration_seconds{handler="/my/super/handler"}`)

	// Register gauge with two labels.
	queueSize = metrics.NewGauge(`queue_size{queue="foobar",topic="baz"}`, func() float64 {
		return float64(foobarQueue.Len())
	})
)

// ...
func requestHandler() {
	startTime := time.Now()
	// ...
	requestsTotal.Inc()
	requestDuration.UpdateDuration(startTime)
}
// ...

// `/metrics` handler for exposing the registered metrics.
http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
	metrics.WritePrometheus(w, true)
})
```

See [docs](http://godoc.org/github.com/VictoriaMetrics/metrics) for more info.


### Users

* `Metrics` has been extracted from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) sources.
  See [this article](https://medium.com/devopslinks/victoriametrics-creating-the-best-remote-storage-for-prometheus-5d92d66787ac)
  for more info about `VictoriaMetrics`.


### FAQ

#### Why the `metrics` API isn't compatible with `github.com/prometheus/client_golang`?

Because the `github.com/prometheus/client_golang` is too complex and is hard to use.


#### Why the `metrics.WritePrometheus` doesn't expose documentation for each metric?

Because this documentation is ignored by Prometheus. The documentation is for users.
Just add comments in the source code or in other suitable place explaining each metric
exposed from your application.
