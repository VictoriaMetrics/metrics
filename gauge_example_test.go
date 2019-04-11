package metrics_test

import (
	"fmt"
	"runtime"

	"github.com/VictoriaMetrics/metrics"
)

func ExampleGauge() {
	// Define a gauge exporting the number of goroutines.
	var g = metrics.NewGauge(`goroutines_count`, func() float64 {
		return float64(runtime.NumGoroutine())
	})

	// Obtain gauge value.
	fmt.Println(g.Get())
}
