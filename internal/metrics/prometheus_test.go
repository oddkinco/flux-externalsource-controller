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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPrometheusRecorder_RecordReconciliation(t *testing.T) {
	// Create a new registry for this test to avoid conflicts
	registry := prometheus.NewRegistry()
	
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
	}
	
	registry.MustRegister(recorder.reconciliationTotal, recorder.reconciliationDuration)

	tests := []struct {
		name        string
		namespace   string
		sourceName  string
		sourceType  string
		success     bool
		duration    time.Duration
		wantCounter float64
	}{
		{
			name:        "successful reconciliation",
			namespace:   "default",
			sourceName:  "test-source",
			sourceType:  "http",
			success:     true,
			duration:    100 * time.Millisecond,
			wantCounter: 1,
		},
		{
			name:        "failed reconciliation",
			namespace:   "default",
			sourceName:  "test-source",
			sourceType:  "http",
			success:     false,
			duration:    50 * time.Millisecond,
			wantCounter: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder.RecordReconciliation(tt.namespace, tt.sourceName, tt.sourceType, tt.success, tt.duration)

			successLabel := "false"
			if tt.success {
				successLabel = "true"
			}

			// Check counter metric
			counter := recorder.reconciliationTotal.WithLabelValues(tt.namespace, tt.sourceName, tt.sourceType, successLabel)
			if got := testutil.ToFloat64(counter); got != tt.wantCounter {
				t.Errorf("RecordReconciliation() counter = %v, want %v", got, tt.wantCounter)
			}

			// Histogram metrics are recorded successfully if no panic occurred
		})
	}
}

func TestPrometheusRecorder_RecordSourceRequest(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	recorder := &PrometheusRecorder{
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
	}
	
	registry.MustRegister(recorder.sourceRequestTotal, recorder.sourceRequestDuration)

	tests := []struct {
		name       string
		sourceType string
		success    bool
		duration   time.Duration
	}{
		{
			name:       "successful HTTP request",
			sourceType: "http",
			success:    true,
			duration:   200 * time.Millisecond,
		},
		{
			name:       "failed HTTP request",
			sourceType: "http",
			success:    false,
			duration:   5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder.RecordSourceRequest(tt.sourceType, tt.success, tt.duration)

			successLabel := "false"
			if tt.success {
				successLabel = "true"
			}

			// Check counter metric
			counter := recorder.sourceRequestTotal.WithLabelValues(tt.sourceType, successLabel)
			if got := testutil.ToFloat64(counter); got != 1 {
				t.Errorf("RecordSourceRequest() counter = %v, want 1", got)
			}

			// Histogram metrics are recorded successfully if no panic occurred
		})
	}
}

func TestPrometheusRecorder_RecordTransformation(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	recorder := &PrometheusRecorder{
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
	}
	
	registry.MustRegister(recorder.transformationTotal, recorder.transformationDuration)

	tests := []struct {
		name     string
		success  bool
		duration time.Duration
	}{
		{
			name:     "successful transformation",
			success:  true,
			duration: 10 * time.Millisecond,
		},
		{
			name:     "failed transformation",
			success:  false,
			duration: 5 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder.RecordTransformation(tt.success, tt.duration)

			successLabel := "false"
			if tt.success {
				successLabel = "true"
			}

			// Check counter metric
			counter := recorder.transformationTotal.WithLabelValues(successLabel)
			if got := testutil.ToFloat64(counter); got != 1 {
				t.Errorf("RecordTransformation() counter = %v, want 1", got)
			}

			// Histogram metrics are recorded successfully if no panic occurred
		})
	}
}

func TestPrometheusRecorder_RecordArtifactOperation(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	recorder := &PrometheusRecorder{
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
	}
	
	registry.MustRegister(recorder.artifactOperationTotal, recorder.artifactOperationDuration)

	tests := []struct {
		name      string
		operation string
		success   bool
		duration  time.Duration
	}{
		{
			name:      "successful package operation",
			operation: "package",
			success:   true,
			duration:  100 * time.Millisecond,
		},
		{
			name:      "successful store operation",
			operation: "store",
			success:   true,
			duration:  500 * time.Millisecond,
		},
		{
			name:      "failed store operation",
			operation: "store",
			success:   false,
			duration:  2 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder.RecordArtifactOperation(tt.operation, tt.success, tt.duration)

			successLabel := "false"
			if tt.success {
				successLabel = "true"
			}

			// Check counter metric
			counter := recorder.artifactOperationTotal.WithLabelValues(tt.operation, successLabel)
			if got := testutil.ToFloat64(counter); got != 1 {
				t.Errorf("RecordArtifactOperation() counter = %v, want 1", got)
			}

			// Histogram metrics are recorded successfully if no panic occurred
		})
	}
}

func TestPrometheusRecorder_ActiveReconciliations(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	recorder := &PrometheusRecorder{
		activeReconciliations: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "externalsource_active_reconciliations",
				Help: "Number of currently active reconciliations",
			},
			[]string{"namespace", "name"},
		),
	}
	
	registry.MustRegister(recorder.activeReconciliations)

	namespace := "default"
	name := "test-source"

	// Initially should be 0
	gauge := recorder.activeReconciliations.WithLabelValues(namespace, name)
	if got := testutil.ToFloat64(gauge); got != 0 {
		t.Errorf("Initial active reconciliations = %v, want 0", got)
	}

	// Increment
	recorder.IncActiveReconciliations(namespace, name)
	if got := testutil.ToFloat64(gauge); got != 1 {
		t.Errorf("After increment active reconciliations = %v, want 1", got)
	}

	// Increment again
	recorder.IncActiveReconciliations(namespace, name)
	if got := testutil.ToFloat64(gauge); got != 2 {
		t.Errorf("After second increment active reconciliations = %v, want 2", got)
	}

	// Decrement
	recorder.DecActiveReconciliations(namespace, name)
	if got := testutil.ToFloat64(gauge); got != 1 {
		t.Errorf("After decrement active reconciliations = %v, want 1", got)
	}

	// Decrement to zero
	recorder.DecActiveReconciliations(namespace, name)
	if got := testutil.ToFloat64(gauge); got != 0 {
		t.Errorf("After final decrement active reconciliations = %v, want 0", got)
	}
}

func TestNewPrometheusRecorder(t *testing.T) {
	// This test verifies that NewPrometheusRecorder creates all metrics without panicking
	recorder := NewPrometheusRecorder()

	if recorder == nil {
		t.Fatal("NewPrometheusRecorder() returned nil")
	}

	// Verify all metrics are initialized
	if recorder.reconciliationTotal == nil {
		t.Error("reconciliationTotal metric not initialized")
	}
	if recorder.reconciliationDuration == nil {
		t.Error("reconciliationDuration metric not initialized")
	}
	if recorder.sourceRequestTotal == nil {
		t.Error("sourceRequestTotal metric not initialized")
	}
	if recorder.sourceRequestDuration == nil {
		t.Error("sourceRequestDuration metric not initialized")
	}
	if recorder.transformationTotal == nil {
		t.Error("transformationTotal metric not initialized")
	}
	if recorder.transformationDuration == nil {
		t.Error("transformationDuration metric not initialized")
	}
	if recorder.artifactOperationTotal == nil {
		t.Error("artifactOperationTotal metric not initialized")
	}
	if recorder.artifactOperationDuration == nil {
		t.Error("artifactOperationDuration metric not initialized")
	}
	if recorder.activeReconciliations == nil {
		t.Error("activeReconciliations metric not initialized")
	}

	// Test that we can record metrics without panicking
	recorder.RecordReconciliation("default", "test", "http", true, 100*time.Millisecond)
	recorder.RecordSourceRequest("http", true, 200*time.Millisecond)
	recorder.RecordTransformation(true, 10*time.Millisecond)
	recorder.RecordArtifactOperation("package", true, 50*time.Millisecond)
	recorder.IncActiveReconciliations("default", "test")
	recorder.DecActiveReconciliations("default", "test")
}

func TestPrometheusRecorder_MetricNames(t *testing.T) {
	// Create a separate registry for this test to avoid conflicts
	registry := prometheus.NewRegistry()
	
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
	}
	
	registry.MustRegister(recorder.reconciliationTotal, recorder.reconciliationDuration)

	// Record some metrics to ensure they work
	recorder.RecordReconciliation("test", "test", "http", true, time.Millisecond)
	
	// Verify we can gather metrics after recording
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics after recording: %v", err)
	}

	// Should have at least some metrics now
	if len(metricFamilies) == 0 {
		t.Error("No metrics found after recording")
	}

	// Check that expected metric names are present
	foundMetrics := make(map[string]bool)
	for _, mf := range metricFamilies {
		foundMetrics[mf.GetName()] = true
	}

	expectedMetrics := []string{
		"externalsource_reconciliation_total",
		"externalsource_reconciliation_duration_seconds",
	}

	for _, expected := range expectedMetrics {
		if !foundMetrics[expected] {
			t.Errorf("Expected metric %s not found", expected)
		}
	}
}