import { useState, useMemo, memo } from 'react';
import { Layout } from './components/layout';
import { Card, CardHeader, CardTitle, CardContent } from './components/ui/index';
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
import { MonitoringSection } from './components/sections/MonitoringSection';
import { LogsSection } from './components/sections/LogsSection';
import { BackupsSection } from './components/sections/BackupsSection';
import { EnhancedSettingsSection } from './components/sections/EnhancedSettingsSection';
import { AuthenticationSection } from './components/sections/AuthenticationSection';
import { SchemaVisualizerSection } from './components/sections/SchemaVisualizerSection';
import { APIDocsSection } from './components/sections/APIDocsSection';
import { StorageSection } from './components/sections/StorageSection';
import { DocumentDBSection } from './components/sections/DocumentDBSection';
import { KeyValueSection } from './components/sections/KeyValueSection';
import { ColumnarSection } from './components/sections/ColumnarSection';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import { LoginPage } from './components/auth/LoginPage';
import { EnhancedSQLEditor } from './components/sql-editor/EnhancedSQLEditor';
import { TableEditor } from './components/table-editor/TableEditor';
import type { SidebarItem } from './components/layout';

const sidebarItems: SidebarItem[] = [
  {
    id: 'dashboard',
    label: 'Dashboard',
    icon: <DashboardIcon />,
    path: '/dashboard'
  },
  {
    id: 'data-browser',
    label: 'Table Editor',
    icon: <DatabaseIcon />,
    path: '/data-browser'
  },
  {
    id: 'document-db',
    label: 'Document Store',
    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>,
    path: '/document-db'
  },
  {
    id: 'key-value',
    label: 'Key-Value Store',
    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" /></svg>,
    path: '/key-value'
  },
  {
    id: 'columnar',
    label: 'Columnar Store',
    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2" /></svg>,
    path: '/columnar'
  },
  {
    id: 'sql-editor',
    label: 'SQL Editor',
    icon: <QueryIcon />,
    path: '/sql-editor'
  },
  {
    id: 'authentication',
    label: 'Authentication',
    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>,
    path: '/authentication'
  },
  {
    id: 'schema',
    label: 'Database Schema',
    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 5a1 1 0 011-1h4a1 1 0 011 1v7a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM14 5a1 1 0 011-1h4a1 1 0 011 1v7a1 1 0 01-1 1h-4a1 1 0 01-1-1V5zM4 16a1 1 0 011-1h4a1 1 0 011 1v3a1 1 0 01-1 1H5a1 1 0 01-1-1v-3zM14 16a1 1 0 011-1h4a1 1 0 011 1v3a1 1 0 01-1 1h-4a1 1 0 01-1-1v-3z" /></svg>,
    path: '/schema'
  },
  {
    id: 'storage',
    label: 'Storage',
    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z" /></svg>,
    path: '/storage'
  },
  {
    id: 'api-docs',
    label: 'API',
    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" /></svg>,
    path: '/api-docs'
  },
  {
    id: 'functions',
    label: 'Functions',
    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>,
    path: '/functions'
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

// Memoized stats card component for better performance
const StatsCard = memo(({ icon, title, value, colorClass }: { 
  icon: React.ReactNode; 
  title: string; 
  value: string; 
  colorClass: string;
}) => (
  <Card>
    <CardContent className="p-6">
      <div className="flex items-center">
        <div className={`p-2 ${colorClass} rounded-lg`}>
          {icon}
        </div>
        <div className="ml-4">
          <p className="text-sm font-medium text-gray-600">{title}</p>
          <p className="text-2xl font-bold text-gray-900">{value}</p>
        </div>
      </div>
    </CardContent>
  </Card>
));
StatsCard.displayName = 'StatsCard';

function AppContent() {
  const { isAuthenticated, isLoading, user, logout } = useAuth();
  const [activeSection, setActiveSection] = useState('dashboard');
  
  // API hooks for real data - only fetch when dashboard is active
  const { data: healthData, loading: healthLoading } = useHealth();
  const { data: systemStats, loading: systemLoading } = useSystemStats();
  const { metrics: realTimeMetrics, connected: metricsConnected } = useRealTimeMetrics();

  const handleSidebarItemClick = (itemId: string) => {
    setActiveSection(itemId);
  };

  // Memoize computed values
  const totalRecords = useMemo(() => 
    systemLoading ? '...' : String(systemStats?.database_stats?.total_records || '0'),
    [systemLoading, systemStats]
  );

  const activeConnections = useMemo(() => 
    systemLoading ? '...' : String(systemStats?.active_connections || '0'),
    [systemLoading, systemStats]
  );

  const cpuUsage = useMemo(() => 
    systemLoading ? '...' : `${Math.round(systemStats?.cpu_usage_percent || 0)}%`,
    [systemLoading, systemStats]
  );

  const memoryUsage = useMemo(() => 
    systemLoading ? '...' : `${Math.round((systemStats?.memory_usage_bytes || 0) / 1024 / 1024)}MB`,
    [systemLoading, systemStats]
  );

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
              <StatsCard
                icon={<DatabaseIcon className="w-6 h-6 text-mantis-600" />}
                title="Total Records"
                value={totalRecords}
                colorClass="bg-mantis-100"
              />
              
              <StatsCard
                icon={<QueryIcon className="w-6 h-6 text-blue-600" />}
                title="Active Connections"
                value={activeConnections}
                colorClass="bg-blue-100"
              />
              
              <StatsCard
                icon={<MonitorIcon className="w-6 h-6 text-yellow-600" />}
                title="CPU Usage"
                value={cpuUsage}
                colorClass="bg-yellow-100"
              />
              
              <StatsCard
                icon={<BackupIcon className="w-6 h-6 text-green-600" />}
                title="Memory Usage"
                value={memoryUsage}
                colorClass="bg-green-100"
              />
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

      case 'data-browser':
        return <TableEditor />;

      case 'document-db':
        return <DocumentDBSection />;

      case 'key-value':
        return <KeyValueSection />;

      case 'columnar':
        return <ColumnarSection />;

      case 'sql-editor':
        return <EnhancedSQLEditor />;

      case 'authentication':
        return <AuthenticationSection />;

      case 'schema':
        return <SchemaVisualizerSection />;

      case 'storage':
        return <StorageSection />;

      case 'api-docs':
        return <APIDocsSection />;

      case 'functions':
        return (
          <Card>
            <CardHeader>
              <CardTitle>Edge Functions</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-center py-12">
                <p className="text-gray-600">Functions management coming soon...</p>
              </div>
            </CardContent>
          </Card>
        );

      case 'monitoring':
        return <MonitoringSection />;

      case 'logs':
        return <LogsSection />;

      case 'backups':
        return <BackupsSection />;

      case 'settings':
        return <EnhancedSettingsSection />;
      
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
                <p className="text-gray-600">
                  This section is under development.
                </p>
              </div>
            </CardContent>
          </Card>
        );
    }
  };

  if (isLoading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-mantis-600 mx-auto"></div>
          <p className="text-gray-600 mt-4">Loading...</p>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <LoginPage />;
  }

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
        },
        user: user || undefined,
        onLogout: logout
      }}
    >
      {renderContent()}
    </Layout>
  );
}

function App() {
  return (
    <AuthProvider>
      <AppContent />
    </AuthProvider>
  );
}

export default App;