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

func ExampleGauge_vec() {
	for i := 0; i < 3; i++ {
		// Dynamically construct metric name and pass it to GetOrCreateGauge.
		name := fmt.Sprintf(`metric{label1=%q, label2="%d"}`, "value1", i)
		iLocal := i
		metrics.GetOrCreateGauge(name, func() float64 {
			return float64(iLocal + 1)
		})
	}

	// Read counter values.
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf(`metric{label1=%q, label2="%d"}`, "value1", i)
		n := metrics.GetOrCreateGauge(name, func() float64 { return 0 }).Get()
		fmt.Println(n)
	}

	// Output:
	// 1
	// 2
	// 3
}
