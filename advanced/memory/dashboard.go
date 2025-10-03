package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Dashboard provides a web interface for memory monitoring
type Dashboard struct {
	cacheManager       *CacheManager
	monitor            *MemoryMonitor
	collector          *MetricsCollector
	healthChecker      *HealthChecker
	performanceTracker *PerformanceTracker
	alertManager       *AlertManager
	server             *http.Server
	config             *DashboardConfig
}

// DashboardConfig holds dashboard configuration
type DashboardConfig struct {
	Port            int
	RefreshInterval time.Duration
	MaxDataPoints   int
	EnableProfiling bool
	EnableDebug     bool
}

// DashboardData represents the data sent to the dashboard
type DashboardData struct {
	Metrics      *Metrics        `json:"metrics"`
	HealthStatus *HealthStatus   `json:"health_status"`
	Alerts       []MemoryAlert   `json:"alerts"`
	CacheStats   CacheStats      `json:"cache_stats"`
	MemoryStats  *MemoryStats    `json:"memory_stats"`
	Performance  PerformanceData `json:"performance"`
	Timestamp    time.Time       `json:"timestamp"`
}

// PerformanceData holds performance metrics for the dashboard
type PerformanceData struct {
	AvgAccessTime time.Duration `json:"avg_access_time"`
	P95AccessTime time.Duration `json:"p95_access_time"`
	P99AccessTime time.Duration `json:"p99_access_time"`
	ThroughputRPS float64       `json:"throughput_rps"`
}

// DefaultDashboardConfig returns default dashboard configuration
func DefaultDashboardConfig() *DashboardConfig {
	return &DashboardConfig{
		Port:            8090,
		RefreshInterval: time.Second * 5,
		MaxDataPoints:   100,
		EnableProfiling: false,
		EnableDebug:     false,
	}
}

// NewDashboard creates a new monitoring dashboard
func NewDashboard(
	cacheManager *CacheManager,
	monitor *MemoryMonitor,
	collector *MetricsCollector,
	healthChecker *HealthChecker,
	performanceTracker *PerformanceTracker,
	alertManager *AlertManager,
	config *DashboardConfig,
) *Dashboard {
	if config == nil {
		config = DefaultDashboardConfig()
	}

	return &Dashboard{
		cacheManager:       cacheManager,
		monitor:            monitor,
		collector:          collector,
		healthChecker:      healthChecker,
		performanceTracker: performanceTracker,
		alertManager:       alertManager,
		config:             config,
	}
}

// Start starts the dashboard web server
func (d *Dashboard) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/metrics", d.handleMetrics)
	mux.HandleFunc("/api/health", d.handleHealth)
	mux.HandleFunc("/api/cache/stats", d.handleCacheStats)
	mux.HandleFunc("/api/memory/stats", d.handleMemoryStats)
	mux.HandleFunc("/api/alerts", d.handleAlerts)
	mux.HandleFunc("/api/performance", d.handlePerformance)
	mux.HandleFunc("/api/dashboard", d.handleDashboard)

	// Cache management endpoints
	mux.HandleFunc("/api/cache/clear", d.handleCacheClear)
	mux.HandleFunc("/api/cache/evict", d.handleCacheEvict)
	mux.HandleFunc("/api/cache/policy", d.handleCachePolicy)

	// Memory management endpoints
	mux.HandleFunc("/api/memory/gc", d.handleForceGC)
	mux.HandleFunc("/api/memory/thresholds", d.handleMemoryThresholds)

	// Static files (dashboard UI)
	mux.HandleFunc("/", d.handleIndex)
	mux.HandleFunc("/dashboard.html", d.handleDashboardHTML)
	mux.HandleFunc("/dashboard.js", d.handleDashboardJS)
	mux.HandleFunc("/dashboard.css", d.handleDashboardCSS)

	// WebSocket endpoint for real-time updates
	mux.HandleFunc("/ws", d.handleWebSocket)

	d.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", d.config.Port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		d.server.Shutdown(context.Background())
	}()

	return d.server.ListenAndServe()
}

// Stop stops the dashboard web server
func (d *Dashboard) Stop() error {
	if d.server != nil {
		return d.server.Shutdown(context.Background())
	}
	return nil
}

// HTTP handlers

func (d *Dashboard) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.collector == nil {
		http.Error(w, "Metrics collector not available", http.StatusServiceUnavailable)
		return
	}

	metrics := d.collector.GetMetrics()
	json.NewEncoder(w).Encode(metrics)
}

func (d *Dashboard) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.healthChecker == nil {
		http.Error(w, "Health checker not available", http.StatusServiceUnavailable)
		return
	}

	health := d.healthChecker.CheckHealth()
	json.NewEncoder(w).Encode(health)
}

func (d *Dashboard) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.cacheManager == nil {
		http.Error(w, "Cache manager not available", http.StatusServiceUnavailable)
		return
	}

	stats := d.cacheManager.GetStats()
	json.NewEncoder(w).Encode(stats)
}

func (d *Dashboard) handleMemoryStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.monitor == nil {
		http.Error(w, "Memory monitor not available", http.StatusServiceUnavailable)
		return
	}

	stats := d.monitor.GetStats()
	json.NewEncoder(w).Encode(stats)
}

func (d *Dashboard) handleAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.alertManager == nil {
		http.Error(w, "Alert manager not available", http.StatusServiceUnavailable)
		return
	}

	alerts := d.alertManager.GetAlerts()
	json.NewEncoder(w).Encode(alerts)
}

func (d *Dashboard) handlePerformance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.performanceTracker == nil {
		http.Error(w, "Performance tracker not available", http.StatusServiceUnavailable)
		return
	}

	avg, p95, p99, rps := d.performanceTracker.GetPerformanceMetrics()
	performance := PerformanceData{
		AvgAccessTime: avg,
		P95AccessTime: p95,
		P99AccessTime: p99,
		ThroughputRPS: rps,
	}

	json.NewEncoder(w).Encode(performance)
}

func (d *Dashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	data := &DashboardData{
		Timestamp: time.Now(),
	}

	if d.collector != nil {
		data.Metrics = d.collector.GetMetrics()
	}

	if d.healthChecker != nil {
		data.HealthStatus = d.healthChecker.CheckHealth()
	}

	if d.alertManager != nil {
		data.Alerts = d.alertManager.GetAlerts()
	}

	if d.cacheManager != nil {
		data.CacheStats = d.cacheManager.GetStats()
	}

	if d.monitor != nil {
		data.MemoryStats = d.monitor.GetStats()
	}

	if d.performanceTracker != nil {
		avg, p95, p99, rps := d.performanceTracker.GetPerformanceMetrics()
		data.Performance = PerformanceData{
			AvgAccessTime: avg,
			P95AccessTime: p95,
			P99AccessTime: p99,
			ThroughputRPS: rps,
		}
	}

	json.NewEncoder(w).Encode(data)
}

func (d *Dashboard) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.cacheManager == nil {
		http.Error(w, "Cache manager not available", http.StatusServiceUnavailable)
		return
	}

	d.cacheManager.Clear(r.Context())

	response := map[string]string{"status": "success", "message": "Cache cleared"}
	json.NewEncoder(w).Encode(response)
}

func (d *Dashboard) handleCacheEvict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.cacheManager == nil {
		http.Error(w, "Cache manager not available", http.StatusServiceUnavailable)
		return
	}

	sizeStr := r.URL.Query().Get("size")
	if sizeStr == "" {
		http.Error(w, "Size parameter required", http.StatusBadRequest)
		return
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid size parameter", http.StatusBadRequest)
		return
	}

	err = d.cacheManager.evict(size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]string{"status": "success", "message": "Cache eviction completed"}
	json.NewEncoder(w).Encode(response)
}

func (d *Dashboard) handleCachePolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.cacheManager == nil {
		http.Error(w, "Cache manager not available", http.StatusServiceUnavailable)
		return
	}

	if r.Method == http.MethodGet {
		stats := d.cacheManager.GetStats()
		response := map[string]string{"policy": stats.EvictionPolicy}
		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method == http.MethodPost {
		policy := r.URL.Query().Get("policy")
		if policy == "" {
			http.Error(w, "Policy parameter required", http.StatusBadRequest)
			return
		}

		err := d.cacheManager.SetEvictionPolicy(policy)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response := map[string]string{"status": "success", "policy": policy}
		json.NewEncoder(w).Encode(response)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (d *Dashboard) handleForceGC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.monitor == nil {
		http.Error(w, "Memory monitor not available", http.StatusServiceUnavailable)
		return
	}

	d.monitor.ForceGC()

	response := map[string]string{"status": "success", "message": "Garbage collection triggered"}
	json.NewEncoder(w).Encode(response)
}

func (d *Dashboard) handleMemoryThresholds(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if d.healthChecker == nil {
		http.Error(w, "Health checker not available", http.StatusServiceUnavailable)
		return
	}

	if r.Method == http.MethodGet {
		json.NewEncoder(w).Encode(d.healthChecker.thresholds)
		return
	}

	if r.Method == http.MethodPost {
		var thresholds HealthThresholds
		if err := json.NewDecoder(r.Body).Decode(&thresholds); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		d.healthChecker.SetThresholds(&thresholds)

		response := map[string]string{"status": "success", "message": "Thresholds updated"}
		json.NewEncoder(w).Encode(response)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/dashboard.html", http.StatusFound)
}

func (d *Dashboard) handleDashboardHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(dashboardHTML))
}

func (d *Dashboard) handleDashboardJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(dashboardJS))
}

func (d *Dashboard) handleDashboardCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write([]byte(dashboardCSS))
}

func (d *Dashboard) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// WebSocket implementation would go here
	// For now, return a simple message
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("WebSocket endpoint - implementation pending"))
}

// Static content for the dashboard
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MantisDB Memory Dashboard</title>
    <link rel="stylesheet" href="/dashboard.css">
</head>
<body>
    <div class="container">
        <header>
            <h1>MantisDB Memory Dashboard</h1>
            <div class="health-indicator" id="health-indicator">
                <span id="health-status">Checking...</span>
            </div>
        </header>
        
        <div class="dashboard-grid">
            <div class="card">
                <h2>Cache Statistics</h2>
                <div class="stats-grid">
                    <div class="stat">
                        <span class="label">Hit Ratio</span>
                        <span class="value" id="hit-ratio">-</span>
                    </div>
                    <div class="stat">
                        <span class="label">Total Size</span>
                        <span class="value" id="cache-size">-</span>
                    </div>
                    <div class="stat">
                        <span class="label">Entries</span>
                        <span class="value" id="cache-entries">-</span>
                    </div>
                    <div class="stat">
                        <span class="label">Evictions</span>
                        <span class="value" id="cache-evictions">-</span>
                    </div>
                </div>
            </div>
            
            <div class="card">
                <h2>Memory Usage</h2>
                <div class="stats-grid">
                    <div class="stat">
                        <span class="label">Memory Usage</span>
                        <span class="value" id="memory-usage">-</span>
                    </div>
                    <div class="stat">
                        <span class="label">Heap Usage</span>
                        <span class="value" id="heap-usage">-</span>
                    </div>
                    <div class="stat">
                        <span class="label">GC Pressure</span>
                        <span class="value" id="gc-pressure">-</span>
                    </div>
                    <div class="stat">
                        <span class="label">System Memory</span>
                        <span class="value" id="system-memory">-</span>
                    </div>
                </div>
            </div>
            
            <div class="card">
                <h2>Performance</h2>
                <div class="stats-grid">
                    <div class="stat">
                        <span class="label">Avg Access Time</span>
                        <span class="value" id="avg-access-time">-</span>
                    </div>
                    <div class="stat">
                        <span class="label">P95 Access Time</span>
                        <span class="value" id="p95-access-time">-</span>
                    </div>
                    <div class="stat">
                        <span class="label">P99 Access Time</span>
                        <span class="value" id="p99-access-time">-</span>
                    </div>
                    <div class="stat">
                        <span class="label">Throughput (RPS)</span>
                        <span class="value" id="throughput-rps">-</span>
                    </div>
                </div>
            </div>
            
            <div class="card">
                <h2>Actions</h2>
                <div class="actions">
                    <button onclick="clearCache()">Clear Cache</button>
                    <button onclick="forceGC()">Force GC</button>
                    <button onclick="refreshData()">Refresh</button>
                </div>
            </div>
        </div>
        
        <div class="card">
            <h2>Recent Alerts</h2>
            <div id="alerts-container">
                <p>No alerts</p>
            </div>
        </div>
    </div>
    
    <script src="/dashboard.js"></script>
</body>
</html>`

const dashboardCSS = `
body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    margin: 0;
    padding: 0;
    background-color: #f5f5f5;
    color: #333;
}

.container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 20px;
}

header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 30px;
    padding: 20px;
    background: white;
    border-radius: 8px;
    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

h1 {
    margin: 0;
    color: #2d5a27;
}

.health-indicator {
    padding: 8px 16px;
    border-radius: 20px;
    font-weight: 500;
}

.health-indicator.healthy {
    background-color: #d4edda;
    color: #155724;
}

.health-indicator.unhealthy {
    background-color: #f8d7da;
    color: #721c24;
}

.dashboard-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 20px;
    margin-bottom: 20px;
}

.card {
    background: white;
    border-radius: 8px;
    padding: 20px;
    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

.card h2 {
    margin: 0 0 20px 0;
    color: #2d5a27;
    font-size: 1.2em;
}

.stats-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 15px;
}

.stat {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
}

.stat .label {
    font-size: 0.9em;
    color: #666;
    margin-bottom: 5px;
}

.stat .value {
    font-size: 1.5em;
    font-weight: 600;
    color: #2d5a27;
}

.actions {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
}

button {
    padding: 10px 20px;
    border: none;
    border-radius: 5px;
    background-color: #2d5a27;
    color: white;
    cursor: pointer;
    font-size: 0.9em;
    transition: background-color 0.2s;
}

button:hover {
    background-color: #1e3d1a;
}

button:disabled {
    background-color: #ccc;
    cursor: not-allowed;
}

#alerts-container {
    max-height: 200px;
    overflow-y: auto;
}

.alert {
    padding: 10px;
    margin: 5px 0;
    border-radius: 5px;
    background-color: #fff3cd;
    border-left: 4px solid #ffc107;
}

.alert.error {
    background-color: #f8d7da;
    border-left-color: #dc3545;
}

@media (max-width: 768px) {
    .dashboard-grid {
        grid-template-columns: 1fr;
    }
    
    .stats-grid {
        grid-template-columns: 1fr;
    }
    
    .actions {
        flex-direction: column;
    }
}
`

const dashboardJS = `
let refreshInterval;

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatDuration(nanoseconds) {
    if (nanoseconds < 1000) return nanoseconds + 'ns';
    if (nanoseconds < 1000000) return (nanoseconds / 1000).toFixed(2) + 'μs';
    if (nanoseconds < 1000000000) return (nanoseconds / 1000000).toFixed(2) + 'ms';
    return (nanoseconds / 1000000000).toFixed(2) + 's';
}

function formatPercentage(value) {
    return (value * 100).toFixed(1) + '%';
}

async function fetchDashboardData() {
    try {
        const response = await fetch('/api/dashboard');
        const data = await response.json();
        updateDashboard(data);
    } catch (error) {
        console.error('Failed to fetch dashboard data:', error);
    }
}

function updateDashboard(data) {
    // Update health indicator
    const healthIndicator = document.getElementById('health-indicator');
    const healthStatus = document.getElementById('health-status');
    
    if (data.health_status) {
        healthStatus.textContent = data.health_status.healthy ? 'Healthy' : 'Unhealthy';
        healthIndicator.className = 'health-indicator ' + (data.health_status.healthy ? 'healthy' : 'unhealthy');
    }
    
    // Update cache stats
    if (data.cache_stats) {
        document.getElementById('hit-ratio').textContent = formatPercentage(data.cache_stats.HitRatio);
        document.getElementById('cache-size').textContent = formatBytes(data.cache_stats.TotalSize);
        document.getElementById('cache-entries').textContent = data.cache_stats.TotalEntries.toLocaleString();
        document.getElementById('cache-evictions').textContent = data.cache_stats.EvictionCount.toLocaleString();
    }
    
    // Update memory stats
    if (data.metrics) {
        document.getElementById('memory-usage').textContent = formatPercentage(data.metrics.MemoryUsage);
        document.getElementById('heap-usage').textContent = formatPercentage(data.metrics.HeapUsage);
        document.getElementById('gc-pressure').textContent = data.metrics.GCPressure.toFixed(2);
        document.getElementById('system-memory').textContent = formatBytes(data.metrics.SystemMemory);
    }
    
    // Update performance stats
    if (data.performance) {
        document.getElementById('avg-access-time').textContent = formatDuration(data.performance.avg_access_time);
        document.getElementById('p95-access-time').textContent = formatDuration(data.performance.p95_access_time);
        document.getElementById('p99-access-time').textContent = formatDuration(data.performance.p99_access_time);
        document.getElementById('throughput-rps').textContent = data.performance.throughput_rps.toFixed(2);
    }
    
    // Update alerts
    updateAlerts(data.alerts || []);
}

function updateAlerts(alerts) {
    const container = document.getElementById('alerts-container');
    
    if (alerts.length === 0) {
        container.innerHTML = '<p>No alerts</p>';
        return;
    }
    
    const alertsHTML = alerts.map(alert => 
        '<div class="alert">' +
        '<strong>' + alert.Name + '</strong>: ' +
        'Threshold ' + (alert.Threshold * 100).toFixed(1) + '%, ' +
        'Current ' + (alert.Current * 100).toFixed(1) + '% ' +
        '<small>(' + new Date(alert.Timestamp).toLocaleTimeString() + ')</small>' +
        '</div>'
    ).join('');
    
    container.innerHTML = alertsHTML;
}

async function clearCache() {
    try {
        const response = await fetch('/api/cache/clear', { method: 'POST' });
        const result = await response.json();
        alert(result.message);
        refreshData();
    } catch (error) {
        alert('Failed to clear cache: ' + error.message);
    }
}

async function forceGC() {
    try {
        const response = await fetch('/api/memory/gc', { method: 'POST' });
        const result = await response.json();
        alert(result.message);
        refreshData();
    } catch (error) {
        alert('Failed to force GC: ' + error.message);
    }
}

function refreshData() {
    fetchDashboardData();
}

function startAutoRefresh() {
    refreshInterval = setInterval(fetchDashboardData, 5000);
}

function stopAutoRefresh() {
    if (refreshInterval) {
        clearInterval(refreshInterval);
    }
}

// Initialize dashboard
document.addEventListener('DOMContentLoaded', function() {
    fetchDashboardData();
    startAutoRefresh();
});

// Cleanup on page unload
window.addEventListener('beforeunload', function() {
    stopAutoRefresh();
});
`
