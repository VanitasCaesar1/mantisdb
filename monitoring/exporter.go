package monitoring

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// ExportFormat represents different export formats
type ExportFormat int

const (
	JSONFormat ExportFormat = iota
	PrometheusFormat
	PlainTextFormat
)

// MetricsExporter handles exporting metrics in different formats
type MetricsExporter struct {
	collector *MetricsCollector
}

// NewMetricsExporter creates a new metrics exporter
func NewMetricsExporter(collector *MetricsCollector) *MetricsExporter {
	return &MetricsExporter{
		collector: collector,
	}
}

// Export exports metrics in the specified format
func (me *MetricsExporter) Export(format ExportFormat, writer io.Writer) error {
	switch format {
	case JSONFormat:
		return me.exportJSON(writer)
	case PrometheusFormat:
		return me.exportPrometheus(writer)
	case PlainTextFormat:
		return me.exportPlainText(writer)
	default:
		return fmt.Errorf("unsupported export format: %d", format)
	}
}

// exportJSON exports metrics in JSON format
func (me *MetricsExporter) exportJSON(writer io.Writer) error {
	metrics := me.collector.GetMetrics()
	summary := me.collector.GetSummaryMetrics()

	export := struct {
		Timestamp time.Time          `json:"timestamp"`
		Summary   map[string]int64   `json:"summary"`
		Metrics   map[string]*Metric `json:"metrics"`
	}{
		Timestamp: time.Now(),
		Summary:   summary,
		Metrics:   metrics,
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(export)
}

// exportPrometheus exports metrics in Prometheus format
func (me *MetricsExporter) exportPrometheus(writer io.Writer) error {
	metrics := me.collector.GetMetrics()

	// Group metrics by name
	metricGroups := make(map[string][]*Metric)
	for _, metric := range metrics {
		metricGroups[metric.Name] = append(metricGroups[metric.Name], metric)
	}

	// Sort metric names for consistent output
	var names []string
	for name := range metricGroups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		metricList := metricGroups[name]
		if len(metricList) == 0 {
			continue
		}

		// Write HELP comment
		fmt.Fprintf(writer, "# HELP %s %s\n", name, me.getMetricDescription(name))

		// Write TYPE comment
		fmt.Fprintf(writer, "# TYPE %s %s\n", name, me.getPrometheusType(metricList[0].Type))

		// Write metric values
		for _, metric := range metricList {
			if len(metric.Labels) > 0 {
				labels := me.formatPrometheusLabels(metric.Labels)
				fmt.Fprintf(writer, "%s{%s} %d %d\n",
					name, labels, metric.Value, metric.Timestamp.Unix()*1000)
			} else {
				fmt.Fprintf(writer, "%s %d %d\n",
					name, metric.Value, metric.Timestamp.Unix()*1000)
			}
		}
		fmt.Fprintln(writer)
	}

	return nil
}

// exportPlainText exports metrics in plain text format
func (me *MetricsExporter) exportPlainText(writer io.Writer) error {
	summary := me.collector.GetSummaryMetrics()

	fmt.Fprintf(writer, "MantisDB Metrics Summary - %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintln(writer, strings.Repeat("=", 50))

	// WAL Metrics
	fmt.Fprintln(writer, "\nWAL Metrics:")
	fmt.Fprintf(writer, "  Writes Total: %d\n", summary["wal_writes_total"])
	fmt.Fprintf(writer, "  Write Errors: %d\n", summary["wal_write_errors_total"])
	fmt.Fprintf(writer, "  Sync Latency: %s\n", time.Duration(summary["wal_sync_latency_ns"]))
	fmt.Fprintf(writer, "  File Rotations: %d\n", summary["wal_file_rotations_total"])
	fmt.Fprintf(writer, "  Recovery Time: %s\n", time.Duration(summary["wal_recovery_time_ns"]))

	// Transaction Metrics
	fmt.Fprintln(writer, "\nTransaction Metrics:")
	fmt.Fprintf(writer, "  Started: %d\n", summary["transactions_started_total"])
	fmt.Fprintf(writer, "  Committed: %d\n", summary["transactions_committed_total"])
	fmt.Fprintf(writer, "  Aborted: %d\n", summary["transactions_aborted_total"])
	fmt.Fprintf(writer, "  Deadlocks: %d\n", summary["transaction_deadlocks_total"])
	fmt.Fprintf(writer, "  Lock Wait Time: %s\n", time.Duration(summary["lock_wait_time_ns"]))

	// Error Metrics
	fmt.Fprintln(writer, "\nError Metrics:")
	fmt.Fprintf(writer, "  Total Errors: %d\n", summary["errors_total"])
	fmt.Fprintf(writer, "  Corruption Events: %d\n", summary["corruption_events_total"])
	fmt.Fprintf(writer, "  Recovery Attempts: %d\n", summary["recovery_attempts_total"])
	fmt.Fprintf(writer, "  Recovery Failures: %d\n", summary["recovery_failures_total"])

	// Performance Metrics
	fmt.Fprintln(writer, "\nPerformance Metrics:")
	fmt.Fprintf(writer, "  Operation Latency: %s\n", time.Duration(summary["operation_latency_ns"]))
	fmt.Fprintf(writer, "  Throughput: %d ops/sec\n", summary["throughput_ops_per_second"])
	fmt.Fprintf(writer, "  Memory Usage: %s\n", me.formatBytes(summary["memory_usage_bytes"]))
	fmt.Fprintf(writer, "  Disk Usage: %s\n", me.formatBytes(summary["disk_usage_bytes"]))

	// Calculate derived metrics
	totalTxns := summary["transactions_started_total"]
	if totalTxns > 0 {
		commitRate := float64(summary["transactions_committed_total"]) / float64(totalTxns) * 100
		abortRate := float64(summary["transactions_aborted_total"]) / float64(totalTxns) * 100
		fmt.Fprintln(writer, "\nDerived Metrics:")
		fmt.Fprintf(writer, "  Commit Rate: %.2f%%\n", commitRate)
		fmt.Fprintf(writer, "  Abort Rate: %.2f%%\n", abortRate)
	}

	walWrites := summary["wal_writes_total"]
	if walWrites > 0 {
		errorRate := float64(summary["wal_write_errors_total"]) / float64(walWrites) * 100
		fmt.Fprintf(writer, "  WAL Error Rate: %.2f%%\n", errorRate)
	}

	return nil
}

// getMetricDescription returns a description for a metric
func (me *MetricsExporter) getMetricDescription(name string) string {
	descriptions := map[string]string{
		"wal_writes_total":             "Total number of WAL write operations",
		"wal_write_errors_total":       "Total number of WAL write errors",
		"wal_sync_duration":            "Duration of WAL sync operations",
		"wal_file_rotations_total":     "Total number of WAL file rotations",
		"wal_recovery_duration":        "Duration of WAL recovery operations",
		"transactions_started_total":   "Total number of transactions started",
		"transactions_committed_total": "Total number of transactions committed",
		"transactions_aborted_total":   "Total number of transactions aborted",
		"transaction_deadlocks_total":  "Total number of transaction deadlocks",
		"lock_wait_duration":           "Duration of lock wait operations",
		"errors_total":                 "Total number of errors by type",
		"corruption_events_total":      "Total number of corruption events",
		"recovery_attempts_total":      "Total number of recovery attempts",
		"recovery_failures_total":      "Total number of recovery failures",
		"operation_duration":           "Duration of database operations",
		"throughput_ops_per_second":    "Current throughput in operations per second",
		"memory_usage_bytes":           "Current memory usage in bytes",
		"disk_usage_bytes":             "Current disk usage in bytes",
	}

	if desc, exists := descriptions[name]; exists {
		return desc
	}
	return "Database metric"
}

// getPrometheusType returns the Prometheus metric type
func (me *MetricsExporter) getPrometheusType(metricType MetricType) string {
	switch metricType {
	case CounterType:
		return "counter"
	case GaugeType:
		return "gauge"
	case HistogramType:
		return "histogram"
	case TimerType:
		return "histogram"
	default:
		return "gauge"
	}
}

// formatPrometheusLabels formats labels for Prometheus format
func (me *MetricsExporter) formatPrometheusLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// formatBytes formats byte values in human-readable format
func (me *MetricsExporter) formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ExportToString exports metrics to a string in the specified format
func (me *MetricsExporter) ExportToString(format ExportFormat) (string, error) {
	var builder strings.Builder
	err := me.Export(format, &builder)
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}
