package metrics_test

import (
	"fmt"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

func ExamplePrometheusHistogram() {
	// Define a histogram in global scope.
	h := metrics.NewPrometheusHistogram(`request_duration_seconds{path="/foo/bar"}`)

	// Update the histogram with the duration of processRequest call.
	startTime := time.Now()
	processRequest()
	h.UpdateDuration(startTime)
}

func ExamplePrometheusHistogram_vec() {
	for i := 0; i < 3; i++ {
		// Dynamically construct metric name and pass it to GetOrCreatePrometheusHistogram.
		name := fmt.Sprintf(`response_size_bytes{path=%q, code=%q}`, "/foo/bar", 200+i)
		response := processRequest()
		metrics.GetOrCreatePrometheusHistogram(name).Update(float64(len(response)))
	}
}
