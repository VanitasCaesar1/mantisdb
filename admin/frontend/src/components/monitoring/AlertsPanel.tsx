import React, { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Badge, Modal } from '../ui';
import { RefreshIcon } from '../icons';
import { formatRelativeTime } from '../../utils';

export interface Alert {
  id: string;
  title: string;
  message: string;
  severity: 'info' | 'warning' | 'critical';
  component: string;
  timestamp: Date;
  acknowledged: boolean;
  resolved: boolean;
  metadata?: Record<string, any>;
}

export interface AlertsPanelProps {
  alerts: Alert[];
  loading?: boolean;
  onRefresh: () => void;
  onAcknowledge: (alertId: string) => void;
  onResolve: (alertId: string) => void;
  onDismiss: (alertId: string) => void;
}

const AlertsPanel: React.FC<AlertsPanelProps> = ({
  alerts,
  loading = false,
  onRefresh,
  onAcknowledge,
  onResolve,
  onDismiss
}) => {
  const [selectedAlert, setSelectedAlert] = useState<Alert | null>(null);
  const [filterSeverity, setFilterSeverity] = useState<string>('');
  const [showResolved, setShowResolved] = useState(false);

  const filteredAlerts = alerts.filter(alert => {
    const matchesSeverity = !filterSeverity || alert.severity === filterSeverity;
    const matchesResolved = showResolved || !alert.resolved;
    return matchesSeverity && matchesResolved;
  });

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'info': return 'info';
      case 'warning': return 'warning';
      case 'critical': return 'danger';
      default: return 'default';
    }
  };

  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case 'info':
        return (
          <svg className="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
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
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      default:
        return null;
    }
  };

  const activeAlerts = alerts.filter(alert => !alert.resolved);
  const criticalCount = activeAlerts.filter(alert => alert.severity === 'critical').length;
  const warningCount = activeAlerts.filter(alert => alert.severity === 'warning').length;
  const infoCount = activeAlerts.filter(alert => alert.severity === 'info').length;

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>System Alerts</CardTitle>
              <p className="text-sm text-gray-600 mt-1">
                {filteredAlerts.length} of {alerts.length} alerts
              </p>
            </div>
            <div className="flex items-center space-x-3">
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
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {/* Alert Summary */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="p-4 bg-red-50 rounded-lg border border-red-200">
                <div className="flex items-center space-x-2">
                  <svg className="w-5 h-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <div>
                    <p className="text-sm font-medium text-red-800">Critical</p>
                    <p className="text-2xl font-bold text-red-900">{criticalCount}</p>
                  </div>
                </div>
              </div>
              
              <div className="p-4 bg-yellow-50 rounded-lg border border-yellow-200">
                <div className="flex items-center space-x-2">
                  <svg className="w-5 h-5 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.732-.833-2.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z" />
                  </svg>
                  <div>
                    <p className="text-sm font-medium text-yellow-800">Warning</p>
                    <p className="text-2xl font-bold text-yellow-900">{warningCount}</p>
                  </div>
                </div>
              </div>
              
              <div className="p-4 bg-blue-50 rounded-lg border border-blue-200">
                <div className="flex items-center space-x-2">
                  <svg className="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <div>
                    <p className="text-sm font-medium text-blue-800">Info</p>
                    <p className="text-2xl font-bold text-blue-900">{infoCount}</p>
                  </div>
                </div>
              </div>
            </div>

            {/* Filters */}
            <div className="flex items-center space-x-4">
              <div className="flex items-center space-x-2">
                <span className="text-sm text-gray-700">Severity:</span>
                <select
                  value={filterSeverity}
                  onChange={(e) => setFilterSeverity(e.target.value)}
                  className="border border-gray-300 rounded px-3 py-1 text-sm"
                >
                  <option value="">All</option>
                  <option value="critical">Critical</option>
                  <option value="warning">Warning</option>
                  <option value="info">Info</option>
                </select>
              </div>
              <label className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  checked={showResolved}
                  onChange={(e) => setShowResolved(e.target.checked)}
                  className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
                />
                <span className="text-sm text-gray-700">Show resolved</span>
              </label>
            </div>

            {/* Alerts List */}
            {loading && alerts.length === 0 ? (
              <div className="text-center py-8">
                <div className="animate-spin w-8 h-8 border-2 border-mantis-600 border-t-transparent rounded-full mx-auto mb-4"></div>
                <p className="text-gray-600">Loading alerts...</p>
              </div>
            ) : filteredAlerts.length === 0 ? (
              <div className="text-center py-8">
                <div className="text-gray-400 mb-4">
                  <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
                <h3 className="text-lg font-medium text-gray-900 mb-2">No alerts</h3>
                <p className="text-gray-600">
                  {filterSeverity || !showResolved
                    ? 'No alerts match your current filters.'
                    : 'All systems are running normally.'
                  }
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {filteredAlerts.map((alert) => (
                  <div
                    key={alert.id}
                    className={`p-4 rounded-lg border-l-4 cursor-pointer transition-colors ${
                      alert.resolved
                        ? 'bg-gray-50 border-gray-400 opacity-60'
                        : alert.severity === 'critical'
                        ? 'bg-red-50 border-red-400 hover:bg-red-100'
                        : alert.severity === 'warning'
                        ? 'bg-yellow-50 border-yellow-400 hover:bg-yellow-100'
                        : 'bg-blue-50 border-blue-400 hover:bg-blue-100'
                    }`}
                    onClick={() => setSelectedAlert(alert)}
                  >
                    <div className="flex items-start justify-between">
                      <div className="flex items-start space-x-3">
                        <div className="flex-shrink-0 mt-0.5">
                          {getSeverityIcon(alert.severity)}
                        </div>
                        <div className="flex-1">
                          <div className="flex items-center space-x-2 mb-1">
                            <h4 className="text-sm font-medium text-gray-900">
                              {alert.title}
                            </h4>
                            <Badge variant={getSeverityColor(alert.severity)} size="sm">
                              {alert.severity}
                            </Badge>
                            {alert.acknowledged && (
                              <Badge variant="info" size="sm">
                                Acknowledged
                              </Badge>
                            )}
                            {alert.resolved && (
                              <Badge variant="success" size="sm">
                                Resolved
                              </Badge>
                            )}
                          </div>
                          <p className="text-sm text-gray-600 mb-2">
                            {alert.message}
                          </p>
                          <div className="flex items-center space-x-4 text-xs text-gray-500">
                            <span>Component: {alert.component}</span>
                            <span>{formatRelativeTime(alert.timestamp)}</span>
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center space-x-2 ml-4">
                        {!alert.resolved && (
                          <>
                            {!alert.acknowledged && (
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  onAcknowledge(alert.id);
                                }}
                              >
                                Acknowledge
                              </Button>
                            )}
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={(e) => {
                                e.stopPropagation();
                                onResolve(alert.id);
                              }}
                            >
                              Resolve
                            </Button>
                          </>
                        )}
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={(e) => {
                            e.stopPropagation();
                            onDismiss(alert.id);
                          }}
                          className="text-red-600 hover:text-red-700"
                        >
                          Dismiss
                        </Button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Alert Details Modal */}
      {selectedAlert && (
        <Modal
          isOpen={true}
          onClose={() => setSelectedAlert(null)}
          title="Alert Details"
          size="lg"
        >
          <div className="space-y-4">
            <div className="flex items-center space-x-2">
              {getSeverityIcon(selectedAlert.severity)}
              <h3 className="text-lg font-semibold text-gray-900">
                {selectedAlert.title}
              </h3>
              <Badge variant={getSeverityColor(selectedAlert.severity)}>
                {selectedAlert.severity}
              </Badge>
            </div>

            <div className="bg-gray-50 p-4 rounded-lg">
              <p className="text-gray-700">{selectedAlert.message}</p>
            </div>

            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span className="font-medium text-gray-600">Component:</span>
                <p className="text-gray-900">{selectedAlert.component}</p>
              </div>
              <div>
                <span className="font-medium text-gray-600">Timestamp:</span>
                <p className="text-gray-900">{selectedAlert.timestamp.toLocaleString()}</p>
              </div>
              <div>
                <span className="font-medium text-gray-600">Status:</span>
                <div className="flex space-x-2">
                  {selectedAlert.acknowledged && (
                    <Badge variant="info" size="sm">Acknowledged</Badge>
                  )}
                  {selectedAlert.resolved && (
                    <Badge variant="success" size="sm">Resolved</Badge>
                  )}
                  {!selectedAlert.acknowledged && !selectedAlert.resolved && (
                    <Badge variant="warning" size="sm">Active</Badge>
                  )}
                </div>
              </div>
            </div>

            {selectedAlert.metadata && Object.keys(selectedAlert.metadata).length > 0 && (
              <div>
                <h4 className="font-medium text-gray-900 mb-2">Additional Details</h4>
                <div className="bg-gray-50 p-4 rounded-lg">
                  <pre className="text-sm text-gray-700 whitespace-pre-wrap">
                    {JSON.stringify(selectedAlert.metadata, null, 2)}
                  </pre>
                </div>
              </div>
            )}

            <div className="flex justify-end space-x-3 pt-4 border-t border-gray-200">
              {!selectedAlert.resolved && (
                <>
                  {!selectedAlert.acknowledged && (
                    <Button
                      variant="secondary"
                      onClick={() => {
                        onAcknowledge(selectedAlert.id);
                        setSelectedAlert(null);
                      }}
                    >
                      Acknowledge
                    </Button>
                  )}
                  <Button
                    variant="primary"
                    onClick={() => {
                      onResolve(selectedAlert.id);
                      setSelectedAlert(null);
                    }}
                  >
                    Resolve
                  </Button>
                </>
              )}
              <Button
                variant="secondary"
                onClick={() => setSelectedAlert(null)}
              >
                Close
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </>
  );
};

export default AlertsPanel;