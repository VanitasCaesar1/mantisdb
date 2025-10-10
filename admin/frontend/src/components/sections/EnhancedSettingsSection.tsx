import { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Input } from '../ui';
import { SettingsIcon } from '../icons';
import { apiClient } from '../../api/client';
import type React from 'react';

interface Config {
  server: {
    port: number;
    host: string;
    admin_port: number;
  };
  database: {
    data_dir: string;
    cache_size: number;
    max_connections: number;
    wal_enabled: boolean;
  };
  security: {
    auth_enabled: boolean;
    session_timeout: number;
    max_login_attempts: number;
  };
  performance: {
    query_timeout: number;
    max_query_size: number;
    enable_query_cache: boolean;
  };
  backup: {
    auto_backup_enabled: boolean;
    backup_interval_hours: number;
    retention_days: number;
  };
  features: {
    [key: string]: boolean;
  };
}

export function EnhancedSettingsSection() {
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState('general');
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    fetchConfig();
  }, []);

  const fetchConfig = async () => {
    try {
      setLoading(true);
      const resp = await apiClient.getConfig();
      if (resp.success) {
        const data: any = resp.data;
        setConfig({
          server: {
            host: 'localhost',
            port: 8081,
            admin_port: 8082,
            ...(data?.server || {})
          },
          database: {
            data_dir: './data',
            cache_size: 100,
            max_connections: 100,
            wal_enabled: true,
            ...(data?.database || {})
          },
          security: {
            auth_enabled: true,
            session_timeout: 3600,
            max_login_attempts: 5,
            ...(data?.security || {})
          },
          performance: {
            query_timeout: 30,
            max_query_size: 10485760,
            enable_query_cache: true,
            ...(data?.performance || {})
          },
          backup: {
            auto_backup_enabled: true,
            backup_interval_hours: 24,
            retention_days: 30,
            ...(data?.backup || {})
          },
          features: data?.features || {}
        });
      }
    } catch (err) {
      console.error('Failed to fetch config:', err);
      setMessage({ type: 'error', text: 'Failed to load configuration' });
    } finally {
      setLoading(false);
    }
  };

  const saveConfig = async () => {
    if (!config) return;

    try {
      setSaving(true);
      const resp = await apiClient.updateConfig(config as any);
      if (resp.success) {
        setMessage({ type: 'success', text: 'Configuration saved successfully. Restart may be required for some changes.' });
        setTimeout(() => setMessage(null), 5000);
      } else {
        throw new Error((resp.error as any) || 'Save failed');
      }
    } catch (err) {
      setMessage({ type: 'error', text: 'Failed to save configuration' });
    } finally {
      setSaving(false);
    }
  };

  const toggleFeature = (featureName: string) => {
    if (!config) return;
    setConfig({
      ...config,
      features: {
        ...config.features,
        [featureName]: !config.features[featureName]
      }
    });
  };

  const tabs = [
    { id: 'general', label: 'General', icon: '⚙️' },
    { id: 'database', label: 'Database', icon: '🗄️' },
    { id: 'security', label: 'Security', icon: '🔒' },
    { id: 'performance', label: 'Performance', icon: '⚡' },
    { id: 'backup', label: 'Backup', icon: '💾' },
    { id: 'features', label: 'Features', icon: '✨' }
  ];

  if (loading) {
    return (
      <Card>
        <CardContent className="p-12">
          <div className="text-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-mantis-600 mx-auto"></div>
            <p className="text-gray-600 mt-2">Loading configuration...</p>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (!config) {
    return (
      <Card>
        <CardContent className="p-12">
          <div className="text-center">
            <SettingsIcon className="w-12 h-12 mx-auto text-gray-400 mb-4" />
            <p className="text-gray-600">Failed to load configuration</p>
            <Button variant="secondary" onClick={fetchConfig} className="mt-4">
              Retry
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {/* Message Banner */}
      {message && (
        <div className={`p-4 rounded-lg ${
          message.type === 'success' ? 'bg-green-50 text-green-800 border border-green-200' : 'bg-red-50 text-red-800 border border-red-200'
        }`}>
          {message.text}
        </div>
      )}

      {/* Tabs */}
      <Card>
        <CardContent className="p-4">
          <div className="flex space-x-2 overflow-x-auto">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors whitespace-nowrap ${
                  activeTab === tab.id
                    ? 'bg-mantis-600 text-white'
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                }`}
              >
                <span className="mr-2">{tab.icon}</span>
                {tab.label}
              </button>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* General Settings */}
      {activeTab === 'general' && (
        <Card>
          <CardHeader>
            <CardTitle>Server Configuration</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Host
                </label>
                <Input
                  type="text"
                  value={config.server.host}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                    ...config,
                    server: { ...config.server, host: e.target.value }
                  })}
                />
                <p className="text-xs text-gray-500 mt-1">Server bind address</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Main Port
                </label>
                <Input
                  type="number"
                  value={config.server.port}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                    ...config,
                    server: { ...config.server, port: parseInt(e.target.value) }
                  })}
                />
                <p className="text-xs text-gray-500 mt-1">API server port</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Admin Port
                </label>
                <Input
                  type="number"
                  value={config.server.admin_port}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                    ...config,
                    server: { ...config.server, admin_port: parseInt(e.target.value) }
                  })}
                />
                <p className="text-xs text-gray-500 mt-1">Admin interface port</p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Database Settings */}
      {activeTab === 'database' && (
        <Card>
          <CardHeader>
            <CardTitle>Database Configuration</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Data Directory
                </label>
                <Input
                  type="text"
                  value={config.database.data_dir}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                    ...config,
                    database: { ...config.database, data_dir: e.target.value }
                  })}
                />
                <p className="text-xs text-gray-500 mt-1">Path to database storage</p>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Cache Size (MB)
                  </label>
                  <Input
                    type="number"
                    value={config.database.cache_size}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                      ...config,
                      database: { ...config.database, cache_size: parseInt(e.target.value) }
                    })}
                  />
                  <p className="text-xs text-gray-500 mt-1">Memory cache allocation</p>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Max Connections
                  </label>
                  <Input
                    type="number"
                    value={config.database.max_connections}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                      ...config,
                      database: { ...config.database, max_connections: parseInt(e.target.value) }
                    })}
                  />
                  <p className="text-xs text-gray-500 mt-1">Concurrent connection limit</p>
                </div>
              </div>
              <div className="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
                <div>
                  <h4 className="font-medium text-gray-900">Write-Ahead Logging (WAL)</h4>
                  <p className="text-sm text-gray-600">Enable WAL for durability</p>
                </div>
                <button
                  onClick={() => setConfig({
                    ...config,
                    database: { ...config.database, wal_enabled: !config.database.wal_enabled }
                  })}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    config.database.wal_enabled ? 'bg-mantis-600' : 'bg-gray-200'
                  }`}
                >
                  <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                    config.database.wal_enabled ? 'translate-x-6' : 'translate-x-1'
                  }`} />
                </button>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Security Settings */}
      {activeTab === 'security' && (
        <Card>
          <CardHeader>
            <CardTitle>Security Configuration</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-6">
              <div className="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
                <div>
                  <h4 className="font-medium text-gray-900">Authentication Enabled</h4>
                  <p className="text-sm text-gray-600">Require login for admin access</p>
                </div>
                <button
                  onClick={() => setConfig({
                    ...config,
                    security: { ...config.security, auth_enabled: !config.security.auth_enabled }
                  })}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    config.security.auth_enabled ? 'bg-mantis-600' : 'bg-gray-200'
                  }`}
                >
                  <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                    config.security.auth_enabled ? 'translate-x-6' : 'translate-x-1'
                  }`} />
                </button>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Session Timeout (seconds)
                  </label>
                  <Input
                    type="number"
                    value={config.security.session_timeout}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                      ...config,
                      security: { ...config.security, session_timeout: parseInt(e.target.value) }
                    })}
                  />
                  <p className="text-xs text-gray-500 mt-1">Auto logout after inactivity</p>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Max Login Attempts
                  </label>
                  <Input
                    type="number"
                    value={config.security.max_login_attempts}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                      ...config,
                      security: { ...config.security, max_login_attempts: parseInt(e.target.value) }
                    })}
                  />
                  <p className="text-xs text-gray-500 mt-1">Before account lockout</p>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Performance Settings */}
      {activeTab === 'performance' && (
        <Card>
          <CardHeader>
            <CardTitle>Performance Configuration</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-6">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Query Timeout (seconds)
                  </label>
                  <Input
                    type="number"
                    value={config.performance.query_timeout}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                      ...config,
                      performance: { ...config.performance, query_timeout: parseInt(e.target.value) }
                    })}
                  />
                  <p className="text-xs text-gray-500 mt-1">Max query execution time</p>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Max Query Size (bytes)
                  </label>
                  <Input
                    type="number"
                    value={config.performance.max_query_size}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                      ...config,
                      performance: { ...config.performance, max_query_size: parseInt(e.target.value) }
                    })}
                  />
                  <p className="text-xs text-gray-500 mt-1">Maximum query payload size</p>
                </div>
              </div>
              <div className="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
                <div>
                  <h4 className="font-medium text-gray-900">Query Cache</h4>
                  <p className="text-sm text-gray-600">Cache query results for better performance</p>
                </div>
                <button
                  onClick={() => setConfig({
                    ...config,
                    performance: { ...config.performance, enable_query_cache: !config.performance.enable_query_cache }
                  })}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    config.performance.enable_query_cache ? 'bg-mantis-600' : 'bg-gray-200'
                  }`}
                >
                  <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                    config.performance.enable_query_cache ? 'translate-x-6' : 'translate-x-1'
                  }`} />
                </button>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Backup Settings */}
      {activeTab === 'backup' && (
        <Card>
          <CardHeader>
            <CardTitle>Backup Configuration</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-6">
              <div className="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
                <div>
                  <h4 className="font-medium text-gray-900">Auto Backup</h4>
                  <p className="text-sm text-gray-600">Automatically create periodic backups</p>
                </div>
                <button
                  onClick={() => setConfig({
                    ...config,
                    backup: { ...config.backup, auto_backup_enabled: !config.backup.auto_backup_enabled }
                  })}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    config.backup.auto_backup_enabled ? 'bg-mantis-600' : 'bg-gray-200'
                  }`}
                >
                  <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                    config.backup.auto_backup_enabled ? 'translate-x-6' : 'translate-x-1'
                  }`} />
                </button>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Backup Interval (hours)
                  </label>
                  <Input
                    type="number"
                    value={config.backup.backup_interval_hours}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                      ...config,
                      backup: { ...config.backup, backup_interval_hours: parseInt(e.target.value) }
                    })}
                  />
                  <p className="text-xs text-gray-500 mt-1">Time between automatic backups</p>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Retention Period (days)
                  </label>
                  <Input
                    type="number"
                    value={config.backup.retention_days}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setConfig({
                      ...config,
                      backup: { ...config.backup, retention_days: parseInt(e.target.value) }
                    })}
                  />
                  <p className="text-xs text-gray-500 mt-1">Keep backups for this many days</p>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Feature Toggles */}
      {activeTab === 'features' && (
        <Card>
          <CardHeader>
            <CardTitle>Feature Toggles</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {Object.entries(config.features).map(([feature, enabled]) => (
                <div key={feature} className="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
                  <div>
                    <h4 className="font-medium text-gray-900 capitalize">
                      {feature.replace(/_/g, ' ')}
                    </h4>
                    <p className="text-sm text-gray-600">
                      {enabled ? 'Enabled' : 'Disabled'}
                    </p>
                  </div>
                  <button
                    onClick={() => toggleFeature(feature)}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                      enabled ? 'bg-mantis-600' : 'bg-gray-200'
                    }`}
                  >
                    <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      enabled ? 'translate-x-6' : 'translate-x-1'
                    }`} />
                  </button>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Save Button */}
      <div className="flex justify-end space-x-4">
        <Button variant="secondary" onClick={fetchConfig}>
          Reset Changes
        </Button>
        <Button onClick={saveConfig} disabled={saving}>
          {saving ? 'Saving...' : 'Save Configuration'}
        </Button>
      </div>
    </div>
  );
}
