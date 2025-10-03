import { useState } from 'react';
import { Layout } from './components/layout';
import { Card, CardHeader, CardTitle, CardContent, Button } from './components/ui';
import { 
  DashboardIcon, 
  DatabaseIcon, 
  QueryIcon, 
  MonitorIcon, 
  BackupIcon, 
  SettingsIcon,
  LogsIcon 
} from './components/icons';
import { useHealth, useSystemStats, useRealTimeMetrics } from './hooks/useApi';
import { DataBrowserSection } from './components/sections/DataBrowserSection';
import type { SidebarItem } from './components/layout';

const sidebarItems: SidebarItem[] = [
  {
    id: 'dashboard',
    label: 'Dashboard',
    icon: <DashboardIcon />,
    path: '/dashboard'
  },
  {
    id: 'data',
    label: 'Data Browser',
    icon: <DatabaseIcon />,
    path: '/data'
  },
  {
    id: 'query',
    label: 'SQL Editor',
    icon: <QueryIcon />,
    path: '/query'
  },
  {
    id: 'monitoring',
    label: 'Monitoring',
    icon: <MonitorIcon />,
    path: '/monitoring',
    badge: 'Live'
  },
  {
    id: 'logs',
    label: 'Logs',
    icon: <LogsIcon />,
    path: '/logs'
  },
  {
    id: 'backups',
    label: 'Backups',
    icon: <BackupIcon />,
    path: '/backups'
  },
  {
    id: 'settings',
    label: 'Settings',
    icon: <SettingsIcon />,
    path: '/settings'
  }
];

function App() {
  const [activeSection, setActiveSection] = useState('dashboard');
  
  // API hooks for real data
  const { data: healthData, loading: healthLoading } = useHealth();
  const { data: systemStats, loading: systemLoading } = useSystemStats();
  const { metrics: realTimeMetrics, connected: metricsConnected } = useRealTimeMetrics();

  const handleSidebarItemClick = (itemId: string) => {
    setActiveSection(itemId);
  };

  const renderContent = () => {
    switch (activeSection) {
      case 'dashboard':
        return (
          <div className="space-y-6">
            {/* Connection Status */}
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <div className={`w-2 h-2 rounded-full ${healthLoading ? 'bg-yellow-400' : healthData ? 'bg-green-400' : 'bg-red-400'}`} />
                <span className="text-sm text-gray-600">
                  {healthLoading ? 'Connecting...' : healthData ? 'Connected to MantisDB' : 'Connection Failed'}
                </span>
              </div>
              <div className="flex items-center space-x-2">
                <div className={`w-2 h-2 rounded-full ${metricsConnected ? 'bg-green-400' : 'bg-gray-400'}`} />
                <span className="text-sm text-gray-600">
                  {metricsConnected ? 'Live Metrics' : 'Static Data'}
                </span>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center">
                    <div className="p-2 bg-mantis-100 rounded-lg">
                      <DatabaseIcon className="w-6 h-6 text-mantis-600" />
                    </div>
                    <div className="ml-4">
                      <p className="text-sm font-medium text-gray-600">Total Records</p>
                      <p className="text-2xl font-bold text-gray-900">
                        {systemLoading ? '...' : systemStats?.database_stats?.total_records || '0'}
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>
              
              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center">
                    <div className="p-2 bg-blue-100 rounded-lg">
                      <QueryIcon className="w-6 h-6 text-blue-600" />
                    </div>
                    <div className="ml-4">
                      <p className="text-sm font-medium text-gray-600">Active Connections</p>
                      <p className="text-2xl font-bold text-gray-900">
                        {systemLoading ? '...' : systemStats?.active_connections || '0'}
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>
              
              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center">
                    <div className="p-2 bg-yellow-100 rounded-lg">
                      <MonitorIcon className="w-6 h-6 text-yellow-600" />
                    </div>
                    <div className="ml-4">
                      <p className="text-sm font-medium text-gray-600">CPU Usage</p>
                      <p className="text-2xl font-bold text-gray-900">
                        {systemLoading ? '...' : `${Math.round(systemStats?.cpu_usage_percent || 0)}%`}
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>
              
              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center">
                    <div className="p-2 bg-green-100 rounded-lg">
                      <BackupIcon className="w-6 h-6 text-green-600" />
                    </div>
                    <div className="ml-4">
                      <p className="text-sm font-medium text-gray-600">Memory Usage</p>
                      <p className="text-2xl font-bold text-gray-900">
                        {systemLoading ? '...' : `${Math.round((systemStats?.memory_usage_bytes || 0) / 1024 / 1024)}MB`}
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader>
                <CardTitle>System Status</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-gray-600">Database Engine</span>
                    <span className={`px-2 py-1 text-xs font-medium rounded-full ${
                      healthData 
                        ? 'bg-mantis-100 text-mantis-800' 
                        : 'bg-red-100 text-red-800'
                    }`}>
                      {healthLoading ? 'Checking...' : healthData ? 'Healthy' : 'Unhealthy'}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-gray-600">Version</span>
                    <span className="px-2 py-1 text-xs font-medium bg-gray-100 text-gray-800 rounded-full">
                      {systemStats?.version || 'Unknown'}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-gray-600">Uptime</span>
                    <span className="px-2 py-1 text-xs font-medium bg-blue-100 text-blue-800 rounded-full">
                      {systemStats?.uptime_seconds ? `${Math.floor(systemStats.uptime_seconds / 3600)}h` : 'Unknown'}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-gray-600">Platform</span>
                    <span className="px-2 py-1 text-xs font-medium bg-gray-100 text-gray-800 rounded-full">
                      {systemStats?.platform || 'Unknown'}
                    </span>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Real-time Metrics */}
            {realTimeMetrics && (
              <Card>
                <CardHeader>
                  <CardTitle>Live Metrics</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div className="text-center">
                      <p className="text-sm text-gray-600">Queries/sec</p>
                      <p className="text-xl font-bold text-mantis-600">
                        {realTimeMetrics.queries_per_second || 0}
                      </p>
                    </div>
                    <div className="text-center">
                      <p className="text-sm text-gray-600">Cache Hit Ratio</p>
                      <p className="text-xl font-bold text-blue-600">
                        {Math.round((realTimeMetrics.cache_hit_ratio || 0) * 100)}%
                      </p>
                    </div>
                    <div className="text-center">
                      <p className="text-sm text-gray-600">Avg Response Time</p>
                      <p className="text-xl font-bold text-yellow-600">
                        {realTimeMetrics.avg_response_time || 0}ms
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            )}
          </div>
        );

      case 'data':
        return <DataBrowserSection />;

      case 'query':
        return (
          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>SQL Editor</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-center py-12">
                  <div className="text-gray-400 mb-4">
                    <QueryIcon className="w-12 h-12 mx-auto" />
                  </div>
                  <h3 className="text-lg font-medium text-gray-900 mb-2">
                    SQL Query Interface
                  </h3>
                  <p className="text-gray-600 mb-6">
                    Execute SQL queries with syntax highlighting, autocomplete, and result visualization.
                    Save frequently used queries and view execution history.
                  </p>
                  <div className="space-y-4">
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 text-left">
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">SQL Editor</h4>
                        <p className="text-sm text-gray-600">
                          Advanced editor with syntax highlighting and keyboard shortcuts
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Query History</h4>
                        <p className="text-sm text-gray-600">
                          Track all executed queries with execution times and results
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Saved Queries</h4>
                        <p className="text-sm text-gray-600">
                          Save and organize frequently used queries with tags
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Result Visualization</h4>
                        <p className="text-sm text-gray-600">
                          View results in table or JSON format with export options
                        </p>
                      </div>
                    </div>
                    <Button variant="secondary">
                      Components Ready - Connect to API
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        );

      case 'monitoring':
        return (
          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>System Monitoring</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-center py-12">
                  <div className="text-gray-400 mb-4">
                    <MonitorIcon className="w-12 h-12 mx-auto" />
                  </div>
                  <h3 className="text-lg font-medium text-gray-900 mb-2">
                    Real-time Monitoring Dashboard
                  </h3>
                  <p className="text-gray-600 mb-6">
                    Monitor system metrics, view logs in real-time, track health checks, and manage alerts.
                    Get insights into database performance and system health.
                  </p>
                  <div className="space-y-4">
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 text-left">
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Metrics Dashboard</h4>
                        <p className="text-sm text-gray-600">
                          Real-time CPU, memory, disk usage and performance metrics
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Log Viewer</h4>
                        <p className="text-sm text-gray-600">
                          Stream logs in real-time with filtering and search capabilities
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Health Checks</h4>
                        <p className="text-sm text-gray-600">
                          Monitor system components and service health status
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Alerts Panel</h4>
                        <p className="text-sm text-gray-600">
                          Manage system alerts with acknowledgment and resolution
                        </p>
                      </div>
                    </div>
                    <Button variant="secondary">
                      Components Ready - Connect to API
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        );

      case 'logs':
        return (
          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>System Logs</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-center py-12">
                  <div className="text-gray-400 mb-4">
                    <LogsIcon className="w-12 h-12 mx-auto" />
                  </div>
                  <h3 className="text-lg font-medium text-gray-900 mb-2">
                    Log Management Interface
                  </h3>
                  <p className="text-gray-600 mb-6">
                    View, search, and filter system logs with real-time streaming capabilities.
                    Monitor application events and troubleshoot issues effectively.
                  </p>
                  <div className="space-y-4">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-left">
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Real-time Streaming</h4>
                        <p className="text-sm text-gray-600">
                          Watch logs as they happen with auto-scroll and live updates
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Advanced Filtering</h4>
                        <p className="text-sm text-gray-600">
                          Filter by log level, component, time range, and search terms
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Structured Display</h4>
                        <p className="text-sm text-gray-600">
                          View structured logs with metadata and contextual information
                        </p>
                      </div>
                    </div>
                    <Button variant="secondary">
                      Components Ready - Connect to API
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        );

      case 'backups':
        return (
          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>Backup Management</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-center py-12">
                  <div className="text-gray-400 mb-4">
                    <BackupIcon className="w-12 h-12 mx-auto" />
                  </div>
                  <h3 className="text-lg font-medium text-gray-900 mb-2">
                    Backup Management System
                  </h3>
                  <p className="text-gray-600 mb-6">
                    Create, schedule, and manage database backups with comprehensive restore capabilities.
                    Ensure data safety with automated backup schedules and monitoring.
                  </p>
                  <div className="space-y-4">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-left">
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Backup Creation</h4>
                        <p className="text-sm text-gray-600">
                          Create manual backups with compression, encryption, and retention options
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Scheduled Backups</h4>
                        <p className="text-sm text-gray-600">
                          Set up automated backup schedules with cron-like expressions
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Restore & Monitor</h4>
                        <p className="text-sm text-gray-600">
                          Monitor backup status and restore from any backup point
                        </p>
                      </div>
                    </div>
                    <Button variant="secondary">
                      Components Ready - Connect to API
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        );

      case 'settings':
        return (
          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>System Configuration</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-center py-12">
                  <div className="text-gray-400 mb-4">
                    <SettingsIcon className="w-12 h-12 mx-auto" />
                  </div>
                  <h3 className="text-lg font-medium text-gray-900 mb-2">
                    Configuration Management
                  </h3>
                  <p className="text-gray-600 mb-6">
                    Manage database configuration, feature toggles, and system settings.
                    Configure server parameters, enable experimental features, and tune performance.
                  </p>
                  <div className="space-y-4">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-left">
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Configuration Editor</h4>
                        <p className="text-sm text-gray-600">
                          Edit server, database, backup, logging, and performance settings
                        </p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Feature Toggles</h4>
                        <p className="text-sm text-gray-600">
                          Enable or disable experimental features and system capabilities
                        </p>
                      </div>
                    </div>
                    <Button variant="secondary">
                      Components Ready - Connect to API
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        );
      
      default:
        return (
          <Card>
            <CardHeader>
              <CardTitle>{sidebarItems.find(item => item.id === activeSection)?.label}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-center py-12">
                <div className="text-gray-400 mb-4">
                  {sidebarItems.find(item => item.id === activeSection)?.icon}
                </div>
                <h3 className="text-lg font-medium text-gray-900 mb-2">
                  {sidebarItems.find(item => item.id === activeSection)?.label} Interface
                </h3>
                <p className="text-gray-600 mb-6">
                  This section will be implemented in the next tasks.
                </p>
                <Button variant="secondary">
                  Coming Soon
                </Button>
              </div>
            </CardContent>
          </Card>
        );
    }
  };

  return (
    <Layout
      sidebarItems={sidebarItems}
      activeSidebarItem={activeSection}
      onSidebarItemClick={handleSidebarItemClick}
      headerProps={{
        title: sidebarItems.find(item => item.id === activeSection)?.label || 'Dashboard',
        subtitle: 'MantisDB Production Admin Interface',
        status: {
          label: healthLoading ? 'Connecting...' : healthData ? 'Online' : 'Offline',
          variant: healthLoading ? 'warning' : healthData ? 'success' : 'danger'
        }
      }}
    >
      {renderContent()}
    </Layout>
  );
}

export default App;