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
)

// MetricsRecorder defines the interface for recording metrics
type MetricsRecorder interface {
	// RecordReconciliation records a reconciliation attempt with its outcome
	RecordReconciliation(namespace, name, sourceType string, success bool, duration time.Duration)
	
	// RecordSourceRequest records a request to an external source
	RecordSourceRequest(sourceType string, success bool, duration time.Duration)
	
	// RecordTransformation records a data transformation attempt
	RecordTransformation(success bool, duration time.Duration)
	
	// RecordArtifactOperation records an artifact storage operation
	RecordArtifactOperation(operation string, success bool, duration time.Duration)
	
	// IncActiveReconciliations increments the count of active reconciliations
	IncActiveReconciliations(namespace, name string)
	
	// DecActiveReconciliations decrements the count of active reconciliations
	DecActiveReconciliations(namespace, name string)
}