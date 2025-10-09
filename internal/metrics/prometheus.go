/*
Copyright (c) 2025 Odd Kin <oddkin@oddkin.co>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// PrometheusRecorder implements MetricsRecorder using Prometheus metrics
type PrometheusRecorder struct {
	reconciliationTotal       *prometheus.CounterVec
	reconciliationDuration    *prometheus.HistogramVec
	sourceRequestTotal        *prometheus.CounterVec
	sourceRequestDuration     *prometheus.HistogramVec
	transformationTotal       *prometheus.CounterVec
	transformationDuration    prometheus.Histogram
	artifactOperationTotal    *prometheus.CounterVec
	artifactOperationDuration *prometheus.HistogramVec
	activeReconciliations     *prometheus.GaugeVec
}

// NewPrometheusRecorder creates a new PrometheusRecorder and registers metrics
func NewPrometheusRecorder() *PrometheusRecorder {
	recorder := &PrometheusRecorder{
		reconciliationTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "externalsource_reconciliation_total",
				Help: "Total number of reconciliations performed",
			},
			[]string{"namespace", "name", "source_type", "success"},
		),
		reconciliationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "externalsource_reconciliation_duration_seconds",
				Help:    "Duration of reconciliation operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"namespace", "name", "source_type", "success"},
		),
		sourceRequestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "externalsource_source_request_total",
				Help: "Total number of requests made to external sources",
			},
			[]string{"source_type", "success"},
		),
		sourceRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "externalsource_source_request_duration_seconds",
				Help:    "Duration of external source requests in seconds",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			},
			[]string{"source_type", "success"},
		),
		transformationTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "externalsource_transformation_total",
				Help: "Total number of data transformations performed",
			},
			[]string{"success"},
		),
		transformationDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "externalsource_transformation_duration_seconds",
				Help:    "Duration of data transformation operations in seconds",
				Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 2.5, 5, 10},
			},
		),
		artifactOperationTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "externalsource_artifact_operation_total",
				Help: "Total number of artifact operations performed",
			},
			[]string{"operation", "success"},
		),
		artifactOperationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "externalsource_artifact_operation_duration_seconds",
				Help:    "Duration of artifact operations in seconds",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
			},
			[]string{"operation", "success"},
		),
		activeReconciliations: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "externalsource_active_reconciliations",
				Help: "Number of currently active reconciliations",
			},
			[]string{"namespace", "name"},
		),
	}

	// Register all metrics with controller-runtime metrics registry
	metrics.Registry.MustRegister(
		recorder.reconciliationTotal,
		recorder.reconciliationDuration,
		recorder.sourceRequestTotal,
		recorder.sourceRequestDuration,
		recorder.transformationTotal,
		recorder.transformationDuration,
		recorder.artifactOperationTotal,
		recorder.artifactOperationDuration,
		recorder.activeReconciliations,
	)

	return recorder
}

// RecordReconciliation records a reconciliation attempt with its outcome
func (r *PrometheusRecorder) RecordReconciliation(namespace, name, sourceType string, success bool, duration time.Duration) {
	successLabel := "false"
	if success {
		successLabel = "true"
	}

	r.reconciliationTotal.WithLabelValues(namespace, name, sourceType, successLabel).Inc()
	r.reconciliationDuration.WithLabelValues(namespace, name, sourceType, successLabel).Observe(duration.Seconds())
}

// RecordSourceRequest records a request to an external source
func (r *PrometheusRecorder) RecordSourceRequest(sourceType string, success bool, duration time.Duration) {
	successLabel := "false"
	if success {
		successLabel = "true"
	}

	r.sourceRequestTotal.WithLabelValues(sourceType, successLabel).Inc()
	r.sourceRequestDuration.WithLabelValues(sourceType, successLabel).Observe(duration.Seconds())
}

// RecordTransformation records a data transformation attempt
func (r *PrometheusRecorder) RecordTransformation(success bool, duration time.Duration) {
	successLabel := "false"
	if success {
		successLabel = "true"
	}

	r.transformationTotal.WithLabelValues(successLabel).Inc()
	r.transformationDuration.Observe(duration.Seconds())
}

// RecordArtifactOperation records an artifact storage operation
func (r *PrometheusRecorder) RecordArtifactOperation(operation string, success bool, duration time.Duration) {
	successLabel := "false"
	if success {
		successLabel = "true"
	}

	r.artifactOperationTotal.WithLabelValues(operation, successLabel).Inc()
	r.artifactOperationDuration.WithLabelValues(operation, successLabel).Observe(duration.Seconds())
}

// IncActiveReconciliations increments the count of active reconciliations
func (r *PrometheusRecorder) IncActiveReconciliations(namespace, name string) {
	r.activeReconciliations.WithLabelValues(namespace, name).Inc()
}

// DecActiveReconciliations decrements the count of active reconciliations
func (r *PrometheusRecorder) DecActiveReconciliations(namespace, name string) {
	r.activeReconciliations.WithLabelValues(namespace, name).Dec()
}
