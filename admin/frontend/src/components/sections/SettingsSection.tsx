import { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Input } from '../ui';
import { SettingsIcon } from '../icons';

interface Config {
  server: {
    port: number;
    host: string;
  };
  database: {
    data_dir: string;
    cache_size: number;
  };
  features: {
    [key: string]: boolean;
  };
}

export function SettingsSection() {
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    fetchConfig();
  }, []);

  const fetchConfig = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/config');
      if (response.ok) {
        const data = await response.json();
        setConfig(data);
      }
    } catch (err) {
      console.error('Failed to fetch config:', err);
    } finally {
      setLoading(false);
    }
  };

  const saveConfig = async () => {
    if (!config) return;

    try {
      setSaving(true);
      const response = await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
      });

      if (response.ok) {
        alert('Configuration saved successfully. Restart may be required for some changes.');
      }
    } catch (err) {
      console.error('Failed to save config:', err);
      alert('Failed to save configuration');
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
      {/* Server Settings */}
      <Card>
        <CardHeader>
          <CardTitle>Server Configuration</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Host
              </label>
              <Input
                type="text"
                value={config.server.host}
                onChange={(e) => setConfig({
                  ...config,
                  server: { ...config.server, host: e.target.value }
                })}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Port
              </label>
              <Input
                type="number"
                value={config.server.port}
                onChange={(e) => setConfig({
                  ...config,
                  server: { ...config.server, port: parseInt(e.target.value) }
                })}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Database Settings */}
      <Card>
        <CardHeader>
          <CardTitle>Database Configuration</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Data Directory
              </label>
              <Input
                type="text"
                value={config.database.data_dir}
                onChange={(e) => setConfig({
                  ...config,
                  database: { ...config.database, data_dir: e.target.value }
                })}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Cache Size (MB)
              </label>
              <Input
                type="number"
                value={config.database.cache_size}
                onChange={(e) => setConfig({
                  ...config,
                  database: { ...config.database, cache_size: parseInt(e.target.value) }
                })}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Feature Toggles */}
      <Card>
        <CardHeader>
          <CardTitle>Feature Toggles</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {Object.entries(config.features).map(([feature, enabled]) => (
              <div key={feature} className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
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
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      enabled ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Save Button */}
      <div className="flex justify-end space-x-4">
        <Button variant="secondary" onClick={fetchConfig}>
          Reset
        </Button>
        <Button onClick={saveConfig} disabled={saving}>
          {saving ? 'Saving...' : 'Save Configuration'}
        </Button>
      </div>
    </div>
  );
}
