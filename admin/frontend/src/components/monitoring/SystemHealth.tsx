import React from 'react';
import { Card, CardHeader, CardTitle, CardContent, Badge } from '../ui';
import { formatRelativeTime } from '../../utils';

export interface HealthCheck {
  name: string;
  status: 'healthy' | 'warning' | 'critical' | 'unknown';
  message: string;
  lastCheck: Date;
  responseTime?: number;
  details?: Record<string, any>;
}

export interface SystemHealthProps {
  healthChecks: HealthCheck[];
  loading?: boolean;
  onRefresh: () => void;
}

const SystemHealth: React.FC<SystemHealthProps> = ({
  healthChecks,
  loading = false,
  onRefresh
}) => {
  const getOverallStatus = (): 'healthy' | 'warning' | 'critical' => {
    if (healthChecks.some(check => check.status === 'critical')) return 'critical';
    if (healthChecks.some(check => check.status === 'warning')) return 'warning';
    return 'healthy';
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'healthy': return 'success';
      case 'warning': return 'warning';
      case 'critical': return 'danger';
      case 'unknown': return 'default';
      default: return 'default';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'healthy':
        return (
          <svg className="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      case 'warning':
        return (
          <svg className="w-5 h-5 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.732-.833-2.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z" />
          </svg>
        );
      case 'critical':
        return (
          <svg className="w-5 h-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      default:
        return (
          <svg className="w-5 h-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
    }
  };

  const overallStatus = getOverallStatus();
  const healthyCount = healthChecks.filter(check => check.status === 'healthy').length;
  const warningCount = healthChecks.filter(check => check.status === 'warning').length;
  const criticalCount = healthChecks.filter(check => check.status === 'critical').length;

  return (
    <div className="space-y-6">
      {/* Overall Status */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>System Health Overview</CardTitle>
            <button
              onClick={onRefresh}
              disabled={loading}
              className="p-2 text-gray-400 hover:text-gray-600 transition-colors"
            >
              <svg className={`w-5 h-5 ${loading ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
          </div>
        </CardHeader>
        <CardContent>
          <div className="flex items-center space-x-4">
            <div className="flex items-center space-x-2">
              {getStatusIcon(overallStatus)}
              <Badge variant={getStatusColor(overallStatus)} size="md">
                {overallStatus.charAt(0).toUpperCase() + overallStatus.slice(1)}
              </Badge>
            </div>
            <div className="flex items-center space-x-6 text-sm text-gray-600">
              <div className="flex items-center space-x-1">
                <div className="w-3 h-3 bg-green-500 rounded-full"></div>
                <span>{healthyCount} Healthy</span>
              </div>
              {warningCount > 0 && (
                <div className="flex items-center space-x-1">
                  <div className="w-3 h-3 bg-yellow-500 rounded-full"></div>
                  <span>{warningCount} Warning</span>
                </div>
              )}
              {criticalCount > 0 && (
                <div className="flex items-center space-x-1">
                  <div className="w-3 h-3 bg-red-500 rounded-full"></div>
                  <span>{criticalCount} Critical</span>
                </div>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Health Checks */}
      <Card>
        <CardHeader>
          <CardTitle>Health Checks</CardTitle>
        </CardHeader>
        <CardContent>
          {loading && healthChecks.length === 0 ? (
            <div className="text-center py-8">
              <div className="animate-spin w-8 h-8 border-2 border-mantis-600 border-t-transparent rounded-full mx-auto mb-4"></div>
              <p className="text-gray-600">Loading health checks...</p>
            </div>
          ) : healthChecks.length === 0 ? (
            <div className="text-center py-8">
              <div className="text-gray-400 mb-4">
                <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <h3 className="text-lg font-medium text-gray-900 mb-2">No Health Checks</h3>
              <p className="text-gray-600">
                No health check data available.
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              {healthChecks.map((check, index) => (
                <div
                  key={index}
                  className={`p-4 rounded-lg border-l-4 ${
                    check.status === 'healthy'
                      ? 'bg-green-50 border-green-400'
                      : check.status === 'warning'
                      ? 'bg-yellow-50 border-yellow-400'
                      : check.status === 'critical'
                      ? 'bg-red-50 border-red-400'
                      : 'bg-gray-50 border-gray-400'
                  }`}
                >
                  <div className="flex items-start justify-between">
                    <div className="flex items-start space-x-3">
                      <div className="flex-shrink-0 mt-0.5">
                        {getStatusIcon(check.status)}
                      </div>
                      <div className="flex-1">
                        <div className="flex items-center space-x-2 mb-1">
                          <h4 className="text-sm font-medium text-gray-900">
                            {check.name}
                          </h4>
                          <Badge variant={getStatusColor(check.status)} size="sm">
                            {check.status}
                          </Badge>
                        </div>
                        <p className="text-sm text-gray-600 mb-2">
                          {check.message}
                        </p>
                        <div className="flex items-center space-x-4 text-xs text-gray-500">
                          <span>
                            Last check: {formatRelativeTime(check.lastCheck)}
                          </span>
                          {check.responseTime && (
                            <span>
                              Response time: {check.responseTime}ms
                            </span>
                          )}
                        </div>
                        {check.details && Object.keys(check.details).length > 0 && (
                          <details className="mt-2">
                            <summary className="text-xs text-gray-500 cursor-pointer hover:text-gray-700">
                              Show details
                            </summary>
                            <div className="mt-2 text-xs">
                              <div className="bg-white p-2 rounded border">
                                {Object.entries(check.details).map(([key, value]) => (
                                  <div key={key} className="flex justify-between py-1">
                                    <span className="font-medium text-gray-600">{key}:</span>
                                    <span className="text-gray-900">
                                      {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                                    </span>
                                  </div>
                                ))}
                              </div>
                            </div>
                          </details>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
};

export default SystemHealth;