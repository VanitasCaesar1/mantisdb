import { Card, CardHeader, CardTitle, CardContent } from '../ui';
import { useSystemStats, useRealTimeMetrics } from '../../hooks/useApi';

export function MonitoringSection() {
  const { data: systemStats, loading: systemLoading } = useSystemStats();
  const { metrics: realTimeMetrics, connected: metricsConnected } = useRealTimeMetrics();

  return (
    <div className="space-y-6">
      {/* Connection Status */}
      <div className="flex items-center space-x-2">
        <div className={`w-2 h-2 rounded-full ${metricsConnected ? 'bg-green-400' : 'bg-gray-400'}`} />
        <span className="text-sm text-gray-600">
          {metricsConnected ? 'Live Metrics Active' : 'Using Static Data'}
        </span>
      </div>

      {/* Real-time Metrics */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <Card>
          <CardContent className="p-6">
            <div className="text-center">
              <p className="text-sm font-medium text-gray-600 mb-2">CPU Usage</p>
              <p className="text-3xl font-bold text-mantis-600">
                {systemLoading ? '...' : `${Math.round(systemStats?.cpu_usage_percent || 0)}%`}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
            <div className="text-center">
              <p className="text-sm font-medium text-gray-600 mb-2">Memory Usage</p>
              <p className="text-3xl font-bold text-blue-600">
                {systemLoading ? '...' : `${Math.round((systemStats?.memory_usage_bytes || 0) / 1024 / 1024)}MB`}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
            <div className="text-center">
              <p className="text-sm font-medium text-gray-600 mb-2">Queries/sec</p>
              <p className="text-3xl font-bold text-yellow-600">
                {realTimeMetrics?.queries_per_second || 0}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
            <div className="text-center">
              <p className="text-sm font-medium text-gray-600 mb-2">Cache Hit Rate</p>
              <p className="text-3xl font-bold text-green-600">
                {Math.round((realTimeMetrics?.cache_hit_ratio || 0) * 100)}%
              </p>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* System Information */}
      <Card>
        <CardHeader>
          <CardTitle>System Information</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-3">
              <div className="flex justify-between">
                <span className="text-sm font-medium text-gray-600">Version</span>
                <span className="text-sm text-gray-900">{systemStats?.version || 'Unknown'}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-sm font-medium text-gray-600">Platform</span>
                <span className="text-sm text-gray-900">{systemStats?.platform || 'Unknown'}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-sm font-medium text-gray-600">Uptime</span>
                <span className="text-sm text-gray-900">
                  {systemStats?.uptime_seconds ? formatUptime(systemStats.uptime_seconds) : 'Unknown'}
                </span>
              </div>
            </div>
            <div className="space-y-3">
              <div className="flex justify-between">
                <span className="text-sm font-medium text-gray-600">Active Connections</span>
                <span className="text-sm text-gray-900">{systemStats?.active_connections || 0}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-sm font-medium text-gray-600">Total Records</span>
                <span className="text-sm text-gray-900">
                  {systemStats?.database_stats?.total_records?.toLocaleString() || 0}
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-sm font-medium text-gray-600">Response Time</span>
                <span className="text-sm text-gray-900">
                  {realTimeMetrics?.avg_response_time || 0}ms
                </span>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Performance Metrics */}
      <Card>
        <CardHeader>
          <CardTitle>Performance Metrics</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div>
              <div className="flex justify-between mb-2">
                <span className="text-sm font-medium text-gray-600">CPU Usage</span>
                <span className="text-sm text-gray-900">
                  {Math.round(systemStats?.cpu_usage_percent || 0)}%
                </span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div
                  className="bg-mantis-600 h-2 rounded-full transition-all duration-300"
                  style={{ width: `${Math.min(systemStats?.cpu_usage_percent || 0, 100)}%` }}
                />
              </div>
            </div>

            <div>
              <div className="flex justify-between mb-2">
                <span className="text-sm font-medium text-gray-600">Memory Usage</span>
                <span className="text-sm text-gray-900">
                  {Math.round((systemStats?.memory_usage_bytes || 0) / 1024 / 1024)}MB
                </span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div
                  className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                  style={{ 
                    width: `${Math.min(((systemStats?.memory_usage_bytes || 0) / (1024 * 1024 * 1024)) * 100, 100)}%` 
                  }}
                />
              </div>
            </div>

            <div>
              <div className="flex justify-between mb-2">
                <span className="text-sm font-medium text-gray-600">Cache Hit Rate</span>
                <span className="text-sm text-gray-900">
                  {Math.round((realTimeMetrics?.cache_hit_ratio || 0) * 100)}%
                </span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div
                  className="bg-green-600 h-2 rounded-full transition-all duration-300"
                  style={{ width: `${(realTimeMetrics?.cache_hit_ratio || 0) * 100}%` }}
                />
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}
