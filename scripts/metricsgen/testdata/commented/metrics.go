package commented

import "github.com/go-kit/kit/metrics"

type Metrics struct {
	// Height of the chain.
	// We expect multi-line comments to parse correctly.
	Field metrics.Gauge
}
