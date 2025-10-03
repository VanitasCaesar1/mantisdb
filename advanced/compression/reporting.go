package compression

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// CompressionReporter handles compression metrics reporting and visualization
type CompressionReporter struct {
	collector       *CompressionMetricsCollector
	monitor         *CompressionMonitor
	reportScheduler *ReportScheduler
	httpServer      *http.Server
	config          *ReporterConfig
	mutex           sync.RWMutex
}

// ReporterConfig configures the compression reporter
type ReporterConfig struct {
	HTTPPort            int           `json:"http_port"`
	ReportInterval      time.Duration `json:"report_interval"`
	RetentionPeriod     time.Duration `json:"retention_period"`
	EnableHTTPEndpoints bool          `json:"enable_http_endpoints"`
	EnableFileReports   bool          `json:"enable_file_reports"`
	ReportDirectory     string        `json:"report_directory"`
}

// ReportScheduler manages scheduled report generation
type ReportScheduler struct {
	reporter  *CompressionReporter
	schedules map[string]*ReportSchedule
	stopChan  chan struct{}
	wg        sync.WaitGroup
	mutex     sync.RWMutex
}

// ReportSchedule defines a scheduled report
type ReportSchedule struct {
	Name        string        `json:"name"`
	Interval    time.Duration `json:"interval"`
	Format      string        `json:"format"`
	Destination string        `json:"destination"`
	Enabled     bool          `json:"enabled"`
	LastRun     time.Time     `json:"last_run"`
	NextRun     time.Time     `json:"next_run"`
}

// CompressionDashboard provides dashboard data for visualization
type CompressionDashboard struct {
	Overview        *DashboardOverview                  `json:"overview"`
	AlgorithmStats  map[string]*AlgorithmDashboardStats `json:"algorithm_stats"`
	TimeSeriesData  map[string][]MetricDataPoint        `json:"time_series_data"`
	Alerts          []CompressionAlert                  `json:"alerts"`
	Recommendations []string                            `json:"recommendations"`
	LastUpdated     time.Time                           `json:"last_updated"`
}

// DashboardOverview provides high-level compression statistics
type DashboardOverview struct {
	TotalDataProcessed      string  `json:"total_data_processed"`
	StorageSavings          string  `json:"storage_savings"`
	AverageCompressionRatio float64 `json:"average_compression_ratio"`
	CompressionEfficiency   string  `json:"compression_efficiency"`
	ActiveAlerts            int     `json:"active_alerts"`
	SystemHealth            string  `json:"system_health"`
}

// AlgorithmDashboardStats provides algorithm-specific dashboard statistics
type AlgorithmDashboardStats struct {
	Name               string  `json:"name"`
	CompressionCount   int64   `json:"compression_count"`
	AverageRatio       float64 `json:"average_ratio"`
	ThroughputMBps     float64 `json:"throughput_mbps"`
	EfficiencyScore    float64 `json:"efficiency_score"`
	RecommendedUseCase string  `json:"recommended_use_case"`
}

// NewCompressionReporter creates a new compression reporter
func NewCompressionReporter(monitor *CompressionMonitor, config *ReporterConfig) *CompressionReporter {
	if config == nil {
		config = &ReporterConfig{
			HTTPPort:            8090,
			ReportInterval:      5 * time.Minute,
			RetentionPeriod:     24 * time.Hour,
			EnableHTTPEndpoints: true,
			EnableFileReports:   false,
			ReportDirectory:     "./reports",
		}
	}

	collector := NewCompressionMetricsCollector(config.ReportInterval)

	reporter := &CompressionReporter{
		collector: collector,
		monitor:   monitor,
		config:    config,
	}

	reporter.reportScheduler = NewReportScheduler(reporter)

	if config.EnableHTTPEndpoints {
		reporter.setupHTTPServer()
	}

	return reporter
}

// NewReportScheduler creates a new report scheduler
func NewReportScheduler(reporter *CompressionReporter) *ReportScheduler {
	scheduler := &ReportScheduler{
		reporter:  reporter,
		schedules: make(map[string]*ReportSchedule),
		stopChan:  make(chan struct{}),
	}

	// Add default schedules
	scheduler.AddSchedule(&ReportSchedule{
		Name:        "hourly_summary",
		Interval:    time.Hour,
		Format:      "json",
		Destination: "memory",
		Enabled:     true,
	})

	scheduler.AddSchedule(&ReportSchedule{
		Name:        "daily_detailed",
		Interval:    24 * time.Hour,
		Format:      "json",
		Destination: "file",
		Enabled:     true,
	})

	go scheduler.start()

	return scheduler
}

// setupHTTPServer sets up HTTP endpoints for metrics
func (cr *CompressionReporter) setupHTTPServer() {
	mux := http.NewServeMux()

	// Metrics endpoints
	mux.HandleFunc("/metrics", cr.handleMetrics)
	mux.HandleFunc("/metrics/dashboard", cr.handleDashboard)
	mux.HandleFunc("/metrics/report", cr.handleReport)
	mux.HandleFunc("/metrics/alerts", cr.handleAlerts)
	mux.HandleFunc("/metrics/algorithms", cr.handleAlgorithms)
	mux.HandleFunc("/metrics/timeseries", cr.handleTimeSeries)

	cr.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cr.config.HTTPPort),
		Handler: mux,
	}

	go func() {
		if err := cr.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error
		}
	}()
}

// handleMetrics handles the /metrics endpoint
func (cr *CompressionReporter) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	metrics := cr.monitor.GetMetrics()
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
		return
	}
}

// handleDashboard handles the /metrics/dashboard endpoint
func (cr *CompressionReporter) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	dashboard := cr.GenerateDashboard()
	if err := json.NewEncoder(w).Encode(dashboard); err != nil {
		http.Error(w, "Failed to encode dashboard", http.StatusInternalServerError)
		return
	}
}

// handleReport handles the /metrics/report endpoint
func (cr *CompressionReporter) handleReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	report := cr.collector.GenerateReport(cr.monitor)
	if err := json.NewEncoder(w).Encode(report); err != nil {
		http.Error(w, "Failed to encode report", http.StatusInternalServerError)
		return
	}
}

// handleAlerts handles the /metrics/alerts endpoint
func (cr *CompressionReporter) handleAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	report := cr.collector.GenerateReport(cr.monitor)
	if err := json.NewEncoder(w).Encode(report.Alerts); err != nil {
		http.Error(w, "Failed to encode alerts", http.StatusInternalServerError)
		return
	}
}

// handleAlgorithms handles the /metrics/algorithms endpoint
func (cr *CompressionReporter) handleAlgorithms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	metrics := cr.monitor.GetMetrics()
	if err := json.NewEncoder(w).Encode(metrics.Algorithms); err != nil {
		http.Error(w, "Failed to encode algorithm metrics", http.StatusInternalServerError)
		return
	}
}

// handleTimeSeries handles the /metrics/timeseries endpoint
func (cr *CompressionReporter) handleTimeSeries(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	metricName := r.URL.Query().Get("metric")
	if metricName == "" {
		http.Error(w, "metric parameter required", http.StatusBadRequest)
		return
	}

	series, exists := cr.collector.GetMetricSeries(metricName)
	if !exists {
		http.Error(w, "metric not found", http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(series); err != nil {
		http.Error(w, "Failed to encode time series", http.StatusInternalServerError)
		return
	}
}

// GenerateDashboard generates dashboard data
func (cr *CompressionReporter) GenerateDashboard() *CompressionDashboard {
	report := cr.collector.GenerateReport(cr.monitor)

	dashboard := &CompressionDashboard{
		Overview:        cr.generateOverview(report),
		AlgorithmStats:  cr.generateAlgorithmStats(report),
		TimeSeriesData:  cr.generateTimeSeriesData(),
		Alerts:          report.Alerts,
		Recommendations: report.Recommendations,
		LastUpdated:     time.Now(),
	}

	return dashboard
}

// generateOverview generates dashboard overview
func (cr *CompressionReporter) generateOverview(report *CompressionReport) *DashboardOverview {
	overview := &DashboardOverview{
		TotalDataProcessed:      cr.formatBytes(report.GlobalMetrics.TotalDataProcessed),
		StorageSavings:          cr.formatBytes(report.GlobalMetrics.StorageSavings),
		AverageCompressionRatio: report.GlobalMetrics.AverageCompressionRatio,
		CompressionEfficiency:   fmt.Sprintf("%.1f%%", report.GlobalMetrics.CompressionEfficiency),
		ActiveAlerts:            len(report.Alerts),
		SystemHealth:            cr.calculateSystemHealth(report),
	}

	return overview
}

// generateAlgorithmStats generates algorithm-specific dashboard statistics
func (cr *CompressionReporter) generateAlgorithmStats(report *CompressionReport) map[string]*AlgorithmDashboardStats {
	stats := make(map[string]*AlgorithmDashboardStats)

	for algo, metrics := range report.AlgorithmMetrics {
		// Calculate throughput
		throughput := 0.0
		if metrics.TotalCompressionTime > 0 {
			mbProcessed := float64(metrics.TotalInputBytes) / (1024 * 1024)
			throughput = mbProcessed / metrics.TotalCompressionTime.Seconds()
		}

		// Calculate efficiency score (combination of ratio and speed)
		efficiencyScore := metrics.AverageRatio * (throughput / 100) // Normalize throughput

		stats[algo] = &AlgorithmDashboardStats{
			Name:               algo,
			CompressionCount:   metrics.CompressionCount,
			AverageRatio:       metrics.AverageRatio,
			ThroughputMBps:     throughput,
			EfficiencyScore:    efficiencyScore,
			RecommendedUseCase: cr.getRecommendedUseCase(algo, metrics.AverageRatio, throughput),
		}
	}

	return stats
}

// generateTimeSeriesData generates time series data for charts
func (cr *CompressionReporter) generateTimeSeriesData() map[string][]MetricDataPoint {
	timeSeriesData := make(map[string][]MetricDataPoint)

	// Get key metrics for time series visualization
	keyMetrics := []string{
		"compression_ratio",
		"compression_throughput",
		"storage_savings",
		"cpu_overhead",
	}

	for _, metricName := range keyMetrics {
		if series, exists := cr.collector.GetMetricSeries(metricName); exists {
			// Return last 100 data points for visualization
			dataPoints := series.DataPoints
			if len(dataPoints) > 100 {
				dataPoints = dataPoints[len(dataPoints)-100:]
			}
			timeSeriesData[metricName] = dataPoints
		}
	}

	return timeSeriesData
}

// calculateSystemHealth calculates overall system health
func (cr *CompressionReporter) calculateSystemHealth(report *CompressionReport) string {
	score := 100.0

	// Deduct points for alerts
	for _, alert := range report.Alerts {
		switch alert.Level {
		case "critical":
			score -= 20
		case "warning":
			score -= 10
		case "info":
			score -= 5
		}
	}

	// Deduct points for low compression efficiency
	if report.GlobalMetrics.CompressionEfficiency < 10 {
		score -= 15
	}

	// Deduct points for high CPU overhead
	if report.GlobalMetrics.CPUOverhead > 20 {
		score -= 10
	}

	if score >= 90 {
		return "Excellent"
	} else if score >= 75 {
		return "Good"
	} else if score >= 60 {
		return "Fair"
	} else if score >= 40 {
		return "Poor"
	} else {
		return "Critical"
	}
}

// getRecommendedUseCase returns recommended use case for an algorithm
func (cr *CompressionReporter) getRecommendedUseCase(algorithm string, ratio, throughput float64) string {
	switch algorithm {
	case "lz4":
		if throughput > 100 {
			return "Real-time data, high-frequency operations"
		}
		return "General purpose, balanced performance"
	case "snappy":
		if throughput > 80 {
			return "Network compression, streaming data"
		}
		return "Moderate compression with good speed"
	case "zstd":
		if ratio > 3.0 {
			return "Cold data, archival storage"
		}
		return "High compression ratio scenarios"
	default:
		return "General purpose"
	}
}

// formatBytes formats byte counts in human-readable format
func (cr *CompressionReporter) formatBytes(bytes int64) string {
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

// AddSchedule adds a report schedule
func (rs *ReportScheduler) AddSchedule(schedule *ReportSchedule) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	schedule.NextRun = time.Now().Add(schedule.Interval)
	rs.schedules[schedule.Name] = schedule
}

// start starts the report scheduler
func (rs *ReportScheduler) start() {
	rs.wg.Add(1)
	defer rs.wg.Done()

	ticker := time.NewTicker(time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rs.processSchedules()
		case <-rs.stopChan:
			return
		}
	}
}

// processSchedules processes scheduled reports
func (rs *ReportScheduler) processSchedules() {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()

	now := time.Now()
	for _, schedule := range rs.schedules {
		if schedule.Enabled && now.After(schedule.NextRun) {
			go rs.executeSchedule(schedule)
		}
	}
}

// executeSchedule executes a scheduled report
func (rs *ReportScheduler) executeSchedule(schedule *ReportSchedule) {
	report := rs.reporter.collector.GenerateReport(rs.reporter.monitor)

	switch schedule.Destination {
	case "file":
		rs.saveReportToFile(report, schedule)
	case "memory":
		// Store in memory cache (implementation depends on requirements)
	default:
		// Log unknown destination
	}

	// Update schedule
	rs.mutex.Lock()
	schedule.LastRun = time.Now()
	schedule.NextRun = schedule.LastRun.Add(schedule.Interval)
	rs.mutex.Unlock()
}

// saveReportToFile saves a report to file
func (rs *ReportScheduler) saveReportToFile(report *CompressionReport, schedule *ReportSchedule) {
	// Implementation would save report to file system
	// This is a placeholder for the actual file saving logic
}

// Stop stops the compression reporter
func (cr *CompressionReporter) Stop() {
	if cr.httpServer != nil {
		cr.httpServer.Close()
	}

	if cr.reportScheduler != nil {
		close(cr.reportScheduler.stopChan)
		cr.reportScheduler.wg.Wait()
	}

	cr.collector.Stop()
}

// WriteReport writes a report to the specified writer
func (cr *CompressionReporter) WriteReport(w io.Writer, format string) error {
	report := cr.collector.GenerateReport(cr.monitor)

	switch format {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}
