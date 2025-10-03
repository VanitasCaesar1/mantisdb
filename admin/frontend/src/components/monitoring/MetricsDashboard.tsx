import React, { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Badge } from '../ui';
import { RefreshIcon } from '../icons';
import { formatNumber, formatBytes, formatDuration } from '../../utils';
import type { SystemMetrics } from '../../types';

export interface MetricsDashboardProps {
  metrics: SystemMetrics[];
  loading?: boolean;
  onRefresh: () => void;
  refreshInterval?: number;
}

const MetricsDashboard: React.FC<MetricsDashboardProps> = ({
  metrics,
  loading = false,
  onRefresh,
  refreshInterval = 5000
}) => {
  const [autoRefresh, setAutoRefresh] = useState(true);

  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(() => {
      onRefresh();
    }, refreshInterval);

    return () => clearInterval(interval);
  }, [autoRefresh, refreshInterval, onRefresh]);

  const latestMetrics = metrics[metrics.length - 1];
  const previousMetrics = metrics[metrics.length - 2];

  const getChangeIndicator = (current: number, previous: number | undefined) => {
    if (!previous) return null;
    
    const change = current - previous;
    const percentChange = (change / previous) * 100;
    
    if (Math.abs(percentChange) < 1) return null;
    
    return (
      <span className={`text-xs ${change > 0 ? 'text-red-600' : 'text-green-600'}`}>
        {change > 0 ? '↑' : '↓'} {Math.abs(percentChange).toFixed(1)}%
      </span>
    );
  };

  const getStatusColor = (value: number, thresholds: { warning: number; critical: number }) => {
    if (value >= thresholds.critical) return 'danger';
    if (value >= thresholds.warning) return 'warning';
    return 'success';
  };

  if (!latestMetrics) {
    return (
      <Card>
        <CardContent>
          <div className="text-center py-12">
            <div className="text-gray-400 mb-4">
              <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
            </div>
            <h3 className="text-lg font-medium text-gray-900 mb-2">No Metrics Available</h3>
            <p className="text-gray-600 mb-4">
              Waiting for system metrics data...
            </p>
            <Button variant="primary" onClick={onRefresh} loading={loading}>
              Load Metrics
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">System Metrics</h2>
          <p className="text-sm text-gray-600 mt-1">
            Last updated: {latestMetrics.timestamp.toLocaleTimeString()}
          </p>
        </div>
        <div className="flex items-center space-x-3">
          <label className="flex items-center space-x-2">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
              className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
            />
            <span className="text-sm text-gray-700">Auto-refresh</span>
          </label>
          <Button
            variant="secondary"
            size="sm"
            onClick={onRefresh}
            loading={loading}
          >
            <RefreshIcon className="w-4 h-4 mr-2" />
            Refresh
          </Button>
        </div>
      </div>

      {/* Key Metrics Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {/* CPU Usage */}
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-600">CPU Usage</p>
                <div className="flex items-center space-x-2">
                  <p className="text-2xl font-bold text-gray-900">
                    {latestMetrics.cpu_usage.toFixed(1)}%
                  </p>
                  {getChangeIndicator(latestMetrics.cpu_usage, previousMetrics?.cpu_usage)}
                </div>
              </div>
              <div className="p-2 bg-blue-100 rounded-lg">
                <svg className="w-6 h-6 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
                </svg>
              </div>
            </div>
            <div className="mt-4">
              <Badge variant={getStatusColor(latestMetrics.cpu_usage, { warning: 70, critical: 90 })} size="sm">
                {latestMetrics.cpu_usage < 70 ? 'Normal' : latestMetrics.cpu_usage < 90 ? 'High' : 'Critical'}
              </Badge>
            </div>
          </CardContent>
        </Card>

        {/* Memory Usage */}
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-600">Memory Usage</p>
                <div className="flex items-center space-x-2">
                  <p className="text-2xl font-bold text-gray-900">
                    {latestMetrics.memory_usage.toFixed(1)}%
                  </p>
                  {getChangeIndicator(latestMetrics.memory_usage, previousMetrics?.memory_usage)}
                </div>
                <p className="text-xs text-gray-500 mt-1">
                  {formatBytes(latestMetrics.memory_total * (latestMetrics.memory_usage / 100))} / {formatBytes(latestMetrics.memory_total)}
                </p>
              </div>
              <div className="p-2 bg-green-100 rounded-lg">
                <svg className="w-6 h-6 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" />
                </svg>
              </div>
            </div>
            <div className="mt-4">
              <Badge variant={getStatusColor(latestMetrics.memory_usage, { warning: 80, critical: 95 })} size="sm">
                {latestMetrics.memory_usage < 80 ? 'Normal' : latestMetrics.memory_usage < 95 ? 'High' : 'Critical'}
              </Badge>
            </div>
          </CardContent>
        </Card>

        {/* Disk Usage */}
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-600">Disk Usage</p>
                <div className="flex items-center space-x-2">
                  <p className="text-2xl font-bold text-gray-900">
                    {latestMetrics.disk_usage.toFixed(1)}%
                  </p>
                  {getChangeIndicator(latestMetrics.disk_usage, previousMetrics?.disk_usage)}
                </div>
                <p className="text-xs text-gray-500 mt-1">
                  {formatBytes(latestMetrics.disk_total * (latestMetrics.disk_usage / 100))} / {formatBytes(latestMetrics.disk_total)}
                </p>
              </div>
              <div className="p-2 bg-yellow-100 rounded-lg">
                <svg className="w-6 h-6 text-yellow-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
              </div>
            </div>
            <div className="mt-4">
              <Badge variant={getStatusColor(latestMetrics.disk_usage, { warning: 85, critical: 95 })} size="sm">
                {latestMetrics.disk_usage < 85 ? 'Normal' : latestMetrics.disk_usage < 95 ? 'High' : 'Critical'}
              </Badge>
            </div>
          </CardContent>
        </Card>

        {/* Active Connections */}
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-600">Active Connections</p>
                <div className="flex items-center space-x-2">
                  <p className="text-2xl font-bold text-gray-900">
                    {formatNumber(latestMetrics.active_connections)}
                  </p>
                  {getChangeIndicator(latestMetrics.active_connections, previousMetrics?.active_connections)}
                </div>
              </div>
              <div className="p-2 bg-purple-100 rounded-lg">
                <svg className="w-6 h-6 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                </svg>
              </div>
            </div>
            <div className="mt-4">
              <Badge variant="info" size="sm">
                Active
              </Badge>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Performance Metrics */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Query Performance */}
        <Card>
          <CardHeader>
            <CardTitle>Query Performance</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-gray-600">Queries per Second</span>
                <div className="flex items-center space-x-2">
                  <span className="text-lg font-semibold text-gray-900">
                    {latestMetrics.queries_per_second.toFixed(1)}
                  </span>
                  {getChangeIndicator(latestMetrics.queries_per_second, previousMetrics?.queries_per_second)}
                </div>
              </div>
              
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-gray-600">Average Latency</span>
                <div className="flex items-center space-x-2">
                  <span className="text-lg font-semibold text-gray-900">
                    {latestMetrics.query_latency.length > 0 
                      ? formatDuration(latestMetrics.query_latency.reduce((a, b) => a + b, 0) / latestMetrics.query_latency.length)
                      : '0ms'
                    }
                  </span>
                </div>
              </div>

              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-gray-600">Cache Hit Ratio</span>
                <div className="flex items-center space-x-2">
                  <span className="text-lg font-semibold text-gray-900">
                    {(latestMetrics.cache_hit_ratio * 100).toFixed(1)}%
                  </span>
                  <Badge variant={latestMetrics.cache_hit_ratio > 0.8 ? 'success' : latestMetrics.cache_hit_ratio > 0.6 ? 'warning' : 'danger'} size="sm">
                    {latestMetrics.cache_hit_ratio > 0.8 ? 'Good' : latestMetrics.cache_hit_ratio > 0.6 ? 'Fair' : 'Poor'}
                  </Badge>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* System Health */}
        <Card>
          <CardHeader>
            <CardTitle>System Health</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-gray-600">Database Status</span>
                <Badge variant="success" size="sm">Healthy</Badge>
              </div>
              
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-gray-600">Backup Status</span>
                <Badge variant="success" size="sm">Up to Date</Badge>
              </div>

              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-gray-600">Replication</span>
                <Badge variant="info" size="sm">Synced</Badge>
              </div>

              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-gray-600">Monitoring</span>
                <Badge variant="success" size="sm">Active</Badge>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Recent Metrics Chart Placeholder */}
      <Card>
        <CardHeader>
          <CardTitle>Metrics Timeline</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-12 text-gray-500">
            <div className="text-gray-400 mb-4">
              <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
            </div>
            <h3 className="text-lg font-medium text-gray-900 mb-2">Charts Coming Soon</h3>
            <p className="text-gray-600">
              Interactive charts will be available when Chart.js integration is complete.
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
};

export default MetricsDashboard;