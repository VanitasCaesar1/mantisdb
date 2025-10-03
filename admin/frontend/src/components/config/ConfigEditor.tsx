import React, { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Input, Badge } from '../ui';
import { RefreshIcon } from '../icons';
import type { DatabaseConfig } from '../../types';

export interface ConfigEditorProps {
  config: DatabaseConfig;
  loading?: boolean;
  onSave: (config: DatabaseConfig) => Promise<void>;
  onRefresh: () => void;
}

const ConfigEditor: React.FC<ConfigEditorProps> = ({
  config,
  loading = false,
  onSave,
  onRefresh
}) => {
  const [editedConfig, setEditedConfig] = useState<DatabaseConfig>(config);
  const [hasChanges, setHasChanges] = useState(false);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [activeSection, setActiveSection] = useState<string>('server');

  useEffect(() => {
    setEditedConfig(config);
    setHasChanges(false);
    setErrors({});
  }, [config]);

  useEffect(() => {
    const configChanged = JSON.stringify(editedConfig) !== JSON.stringify(config);
    setHasChanges(configChanged);
  }, [editedConfig, config]);

  const validateConfig = (): boolean => {
    const newErrors: Record<string, string> = {};

    // Server validation
    if (editedConfig.server.port < 1 || editedConfig.server.port > 65535) {
      newErrors['server.port'] = 'Port must be between 1 and 65535';
    }
    if (editedConfig.server.admin_port < 1 || editedConfig.server.admin_port > 65535) {
      newErrors['server.admin_port'] = 'Admin port must be between 1 and 65535';
    }
    if (editedConfig.server.port === editedConfig.server.admin_port) {
      newErrors['server.admin_port'] = 'Admin port must be different from server port';
    }

    // Database validation
    if (!editedConfig.database.data_dir.trim()) {
      newErrors['database.data_dir'] = 'Data directory is required';
    }
    if (!editedConfig.database.wal_dir.trim()) {
      newErrors['database.wal_dir'] = 'WAL directory is required';
    }

    // Backup validation
    if (editedConfig.backup.retention_days < 1 || editedConfig.backup.retention_days > 365) {
      newErrors['backup.retention_days'] = 'Retention days must be between 1 and 365';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSave = async () => {
    if (!validateConfig()) {
      return;
    }

    setSaving(true);
    try {
      await onSave(editedConfig);
      setHasChanges(false);
    } catch (error) {
      console.error('Failed to save configuration:', error);
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    setEditedConfig(config);
    setHasChanges(false);
    setErrors({});
  };

  const updateConfig = (path: string, value: any) => {
    const keys = path.split('.');
    const newConfig = { ...editedConfig };
    let current: any = newConfig;
    
    for (let i = 0; i < keys.length - 1; i++) {
      current = current[keys[i]];
    }
    current[keys[keys.length - 1]] = value;
    
    setEditedConfig(newConfig);
    
    // Clear error for this field
    if (errors[path]) {
      const newErrors = { ...errors };
      delete newErrors[path];
      setErrors(newErrors);
    }
  };

  const sections = [
    { id: 'server', label: 'Server', icon: '🖥️' },
    { id: 'database', label: 'Database', icon: '🗄️' },
    { id: 'backup', label: 'Backup', icon: '💾' },
    { id: 'logging', label: 'Logging', icon: '📝' },
    { id: 'memory', label: 'Memory', icon: '🧠' },
    { id: 'compression', label: 'Compression', icon: '🗜️' }
  ];

  const renderServerConfig = () => (
    <div className="space-y-4">
      <h3 className="text-lg font-medium text-gray-900">Server Configuration</h3>
      
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Input
          label="Server Port"
          type="number"
          value={editedConfig.server.port}
          onChange={(e) => updateConfig('server.port', parseInt(e.target.value))}
          error={errors['server.port']}
          min={1}
          max={65535}
        />
        
        <Input
          label="Admin Port"
          type="number"
          value={editedConfig.server.admin_port}
          onChange={(e) => updateConfig('server.admin_port', parseInt(e.target.value))}
          error={errors['server.admin_port']}
          min={1}
          max={65535}
        />
        
        <Input
          label="Host"
          value={editedConfig.server.host}
          onChange={(e) => updateConfig('server.host', e.target.value)}
          placeholder="localhost"
        />
      </div>
    </div>
  );

  const renderDatabaseConfig = () => (
    <div className="space-y-4">
      <h3 className="text-lg font-medium text-gray-900">Database Configuration</h3>
      
      <div className="space-y-4">
        <Input
          label="Data Directory"
          value={editedConfig.database.data_dir}
          onChange={(e) => updateConfig('database.data_dir', e.target.value)}
          error={errors['database.data_dir']}
          placeholder="./data"
        />
        
        <Input
          label="WAL Directory"
          value={editedConfig.database.wal_dir}
          onChange={(e) => updateConfig('database.wal_dir', e.target.value)}
          error={errors['database.wal_dir']}
          placeholder="./wal"
        />
        
        <Input
          label="Cache Size"
          value={editedConfig.database.cache_size}
          onChange={(e) => updateConfig('database.cache_size', e.target.value)}
          placeholder="1GB"
          helperText="Specify size with units (MB, GB)"
        />
      </div>
    </div>
  );

  const renderBackupConfig = () => (
    <div className="space-y-4">
      <h3 className="text-lg font-medium text-gray-900">Backup Configuration</h3>
      
      <div className="space-y-4">
        <label className="flex items-center space-x-2">
          <input
            type="checkbox"
            checked={editedConfig.backup.enabled}
            onChange={(e) => updateConfig('backup.enabled', e.target.checked)}
            className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
          />
          <span className="text-sm font-medium text-gray-700">Enable automatic backups</span>
        </label>
        
        {editedConfig.backup.enabled && (
          <>
            <Input
              label="Schedule (Cron Expression)"
              value={editedConfig.backup.schedule}
              onChange={(e) => updateConfig('backup.schedule', e.target.value)}
              placeholder="0 2 * * *"
              helperText="Daily at 2 AM"
            />
            
            <Input
              label="Retention Days"
              type="number"
              value={editedConfig.backup.retention_days}
              onChange={(e) => updateConfig('backup.retention_days', parseInt(e.target.value))}
              error={errors['backup.retention_days']}
              min={1}
              max={365}
            />
            
            <Input
              label="Destination"
              value={editedConfig.backup.destination}
              onChange={(e) => updateConfig('backup.destination', e.target.value)}
              placeholder="./backups"
            />
          </>
        )}
      </div>
    </div>
  );

  const renderLoggingConfig = () => (
    <div className="space-y-4">
      <h3 className="text-lg font-medium text-gray-900">Logging Configuration</h3>
      
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            Log Level
          </label>
          <select
            value={editedConfig.logging.level}
            onChange={(e) => updateConfig('logging.level', e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-2"
          >
            <option value="DEBUG">DEBUG</option>
            <option value="INFO">INFO</option>
            <option value="WARN">WARN</option>
            <option value="ERROR">ERROR</option>
            <option value="FATAL">FATAL</option>
          </select>
        </div>
        
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            Log Format
          </label>
          <select
            value={editedConfig.logging.format}
            onChange={(e) => updateConfig('logging.format', e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-2"
          >
            <option value="json">JSON</option>
            <option value="text">Text</option>
          </select>
        </div>
        
        <Input
          label="Log Output"
          value={editedConfig.logging.output}
          onChange={(e) => updateConfig('logging.output', e.target.value)}
          placeholder="stdout"
        />
      </div>
    </div>
  );

  const renderMemoryConfig = () => (
    <div className="space-y-4">
      <h3 className="text-lg font-medium text-gray-900">Memory Configuration</h3>
      
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Input
          label="Cache Limit"
          value={editedConfig.memory.cache_limit}
          onChange={(e) => updateConfig('memory.cache_limit', e.target.value)}
          placeholder="512MB"
          helperText="Maximum memory for cache"
        />
        
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            Eviction Policy
          </label>
          <select
            value={editedConfig.memory.eviction_policy}
            onChange={(e) => updateConfig('memory.eviction_policy', e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-2"
          >
            <option value="LRU">LRU (Least Recently Used)</option>
            <option value="LFU">LFU (Least Frequently Used)</option>
            <option value="TTL">TTL (Time To Live)</option>
          </select>
        </div>
      </div>
    </div>
  );

  const renderCompressionConfig = () => (
    <div className="space-y-4">
      <h3 className="text-lg font-medium text-gray-900">Compression Configuration</h3>
      
      <div className="space-y-4">
        <label className="flex items-center space-x-2">
          <input
            type="checkbox"
            checked={editedConfig.compression.enabled}
            onChange={(e) => updateConfig('compression.enabled', e.target.checked)}
            className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
          />
          <span className="text-sm font-medium text-gray-700">Enable compression</span>
        </label>
        
        {editedConfig.compression.enabled && (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Compression Algorithm
              </label>
              <select
                value={editedConfig.compression.algorithm}
                onChange={(e) => updateConfig('compression.algorithm', e.target.value)}
                className="w-full border border-gray-300 rounded px-3 py-2"
              >
                <option value="LZ4">LZ4 (Fast)</option>
                <option value="Snappy">Snappy (Balanced)</option>
                <option value="ZSTD">ZSTD (High Compression)</option>
              </select>
            </div>
            
            <Input
              label="Cold Data Threshold"
              value={editedConfig.compression.cold_data_threshold}
              onChange={(e) => updateConfig('compression.cold_data_threshold', e.target.value)}
              placeholder="7d"
              helperText="Time before data is considered cold"
            />
          </div>
        )}
      </div>
    </div>
  );

  const renderSection = () => {
    switch (activeSection) {
      case 'server': return renderServerConfig();
      case 'database': return renderDatabaseConfig();
      case 'backup': return renderBackupConfig();
      case 'logging': return renderLoggingConfig();
      case 'memory': return renderMemoryConfig();
      case 'compression': return renderCompressionConfig();
      default: return null;
    }
  };

  return (
    <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
      {/* Sidebar */}
      <div className="lg:col-span-1">
        <Card>
          <CardHeader>
            <CardTitle>Configuration Sections</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <nav className="space-y-1">
              {sections.map((section) => (
                <button
                  key={section.id}
                  onClick={() => setActiveSection(section.id)}
                  className={`w-full flex items-center space-x-3 px-4 py-3 text-left transition-colors ${
                    activeSection === section.id
                      ? 'bg-mantis-50 text-mantis-700 border-r-2 border-mantis-500'
                      : 'text-gray-700 hover:bg-gray-50'
                  }`}
                >
                  <span className="text-lg">{section.icon}</span>
                  <span className="font-medium">{section.label}</span>
                </button>
              ))}
            </nav>
          </CardContent>
        </Card>
      </div>

      {/* Main Content */}
      <div className="lg:col-span-3">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <CardTitle>Configuration Editor</CardTitle>
                {hasChanges && (
                  <Badge variant="warning" size="sm">
                    Unsaved Changes
                  </Badge>
                )}
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
                {hasChanges && (
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleReset}
                  >
                    Reset
                  </Button>
                )}
                <Button
                  variant="primary"
                  size="sm"
                  onClick={handleSave}
                  loading={saving}
                  disabled={!hasChanges || Object.keys(errors).length > 0}
                >
                  Save Changes
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="text-center py-8">
                <div className="animate-spin w-8 h-8 border-2 border-mantis-600 border-t-transparent rounded-full mx-auto mb-4"></div>
                <p className="text-gray-600">Loading configuration...</p>
              </div>
            ) : (
              <div className="space-y-6">
                {renderSection()}
                
                {Object.keys(errors).length > 0 && (
                  <div className="bg-red-50 border border-red-200 rounded-lg p-4">
                    <h4 className="font-medium text-red-800 mb-2">Configuration Errors</h4>
                    <ul className="text-sm text-red-700 space-y-1">
                      {Object.entries(errors).map(([field, error]) => (
                        <li key={field}>• {error}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
};

export default ConfigEditor;