//revive:disable:dot-imports
package observability_test

import (
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// findMetricByName searches for a metric by name in the collected data.
func findMetricByName(rm metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i]
			}
		}
	}
	return nil
}
