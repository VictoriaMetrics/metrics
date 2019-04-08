[![Build Status](https://travis-ci.org/VictoriaMetrics/metrics.svg)](https://travis-ci.org/VictoriaMetrics/metrics)
[![GoDoc](https://godoc.org/github.com/VictoriaMetrics/metrics?status.svg)](http://godoc.org/github.com/VictoriaMetrics/metrics)
[![Go Report](https://goreportcard.com/badge/github.com/VictoriaMetrics/metrics)](https://goreportcard.com/report/github.com/VictoriaMetrics/metrics)
[![codecov](https://codecov.io/gh/VictoriaMetrics/metrics/branch/master/graph/badge.svg)](https://codecov.io/gh/VictoriaMetrics/metrics)

# metrics - lightweight alternative to `github.com/prometheus/client_golang/prometheus`.


### Features

* Lightweight. Has minimal number of third-party dependencies and all these deps are small.
  See [this article](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d) for details.
* Easy to use. See the [API docs](http://godoc.org/github.com/VictoriaMetrics/metrics).
* Fast.


### Limitations

* It doesn't implement advanced functionality from [github.com/prometheus/client_golang/prometheus](https://godoc.org/github.com/prometheus/client_golang/prometheus).


### Users

* `Metrics` has been extracted from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) sources.
  See [this article](https://medium.com/devopslinks/victoriametrics-creating-the-best-remote-storage-for-prometheus-5d92d66787ac)
  for more info about `VictoriaMetrics`.
