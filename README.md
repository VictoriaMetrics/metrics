[![GoDoc](https://godoc.org/github.com/VictoriaMetrics/metrics?status.svg)](http://godoc.org/github.com/VictoriaMetrics/metrics)
[![Go Report](https://goreportcard.com/badge/github.com/VictoriaMetrics/metrics)](https://goreportcard.com/report/github.com/VictoriaMetrics/metrics)

# metrics - lightweight package for exporting metrics in Prometheus format


### Features

* Lightweight. Has minimal number of third-party dependencies and all these deps are small.
  See [this article](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d) for details.
* Easy to use. See the [API docs](http://godoc.org/github.com/VictoriaMetrics/metrics).
* Fast.


### Limitations

* It doesn't implement advanced functionality from [github.com/prometheus/client_golang/prometheus](https://godoc.org/github.com/prometheus/client_golang/prometheus).


### Usage

```go
import "github.com/VictoriaMetrics/metrics"
// ...
var (
	requestsTotal = metrics.NewCounter("requests_total")

	queueSize = metrics.NewGauge(`queue_size{queue="foobar"}`, func() float64 {
		return float64(foobarQueue.Len())
	})

	requestDuration = metrics.NewSummary(`requests_duration_seconds{handler="/my/super/handler"}`)
)
// ...
func requestHandler() {
	startTime := time.Now()
	// ...
	requestsTotal.Inc()
	requestDuration.UpdateDuration(startTime)
}
```

See [docs](http://godoc.org/github.com/VictoriaMetrics/metrics) for more info.


### Users

* `Metrics` has been extracted from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) sources.
  See [this article](https://medium.com/devopslinks/victoriametrics-creating-the-best-remote-storage-for-prometheus-5d92d66787ac)
  for more info about `VictoriaMetrics`.
