package metrics_test

import (
	"fmt"
	"github.com/VictoriaMetrics/metrics"
)

func ExampleFloatCounter() {
	// Define a float64 counter in global scope.
	var fc = metrics.NewFloatCounter(`float_metric_total{label1="value1", label2="value2"}`)

	// Add to the counter when needed.
	for range 10 {
		fc.Add(1.01)
	}
	n := fc.Get()
	fmt.Println(n)

	// Output:
	// 10.1
}

func ExampleFloatCounter_vec() {
	for i := range 3 {
		// Dynamically construct metric name and pass it to GetOrCreateFloatCounter.
		name := fmt.Sprintf(`float_metric_total{label1=%q, label2="%d"}`, "value1", i)
		metrics.GetOrCreateFloatCounter(name).Add(float64(i) + 1.01)
	}

	// Read counter values.
	for i := range 3 {
		name := fmt.Sprintf(`float_metric_total{label1=%q, label2="%d"}`, "value1", i)
		n := metrics.GetOrCreateFloatCounter(name).Get()
		fmt.Println(n)
	}

	// Output:
	// 1.01
	// 2.01
	// 3.01
}
