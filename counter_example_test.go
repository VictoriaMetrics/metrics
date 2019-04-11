package metrics_test

import (
	"fmt"
	"github.com/VictoriaMetrics/metrics"
)

func ExampleCounter() {
	// Define a counter in global scope.
	var c = metrics.NewCounter(`metric_total{label1="value1", label2="value2"}`)

	// Increment the counter when needed.
	for i := 0; i < 10; i++ {
		c.Inc()
	}
	n := c.Get()
	fmt.Println(n)

	// Output:
	// 10
}

func ExampleCounter_vec() {
	for i := 0; i < 3; i++ {
		// Dynamically construct metric name and pass it to GetOrCreateCounter.
		name := fmt.Sprintf(`metric_total{label1=%q, label2="%d"}`, "value1", i)
		metrics.GetOrCreateCounter(name).Add(i + 1)
	}

	// Read counter values.
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf(`metric_total{label1=%q, label2="%d"}`, "value1", i)
		n := metrics.GetOrCreateCounter(name).Get()
		fmt.Println(n)
	}

	// Output:
	// 1
	// 2
	// 3
}
