import { useState, useEffect } from 'react';
import { Key, Plus, Trash2, RefreshCw, Search, Edit2, Copy, Check, Clock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '../ui';

interface KeyValuePair {
  key: string;
  value: string;
  ttl?: number;
  created_at?: string;
  updated_at?: string;
}

export const KeyValueSection = () => {
  const [keys, setKeys] = useState<KeyValuePair[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>('');
  const [searchQuery, setSearchQuery] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editingKey, setEditingKey] = useState<KeyValuePair | null>(null);
  const [formData, setFormData] = useState({ key: '', value: '', ttl: '' });
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [stats, setStats] = useState<any>(null);

  useEffect(() => {
    loadKeys();
    loadStats();
  }, []);

  const loadKeys = async () => {
    setLoading(true);
    setError('');
    
    try {
      const response = await fetch('/api/kv/query?prefix=&limit=100');
      if (response.ok) {
        const data = await response.json();
        setKeys(data.keys || []);
      } else {
        setError('Failed to load keys');
      }
    } catch (err) {
      console.error('Failed to load keys:', err);
      setError('Failed to load keys');
    } finally {
      setLoading(false);
    }
  };

  const loadStats = async () => {
    try {
      const response = await fetch('/api/kv/stats');
      if (response.ok) {
        const data = await response.json();
        // Normalize stats for UI
        setStats({
          total_keys: data.total_keys ?? 0,
          memory_usage: data.memory_usage_estimate ?? 0,
          expired_keys: data.expired_keys ?? 0,
          hit_rate: data.hit_rate ?? null,
          store_type: data.store_type ?? 'key-value',
        });
      }
    } catch (err) {
      console.error('Failed to load stats:', err);
    }
  };

  const handleSave = async () => {
    if (!formData.key || !formData.value) {
      setError('Key and value are required');
      return;
    }

    try {
      const body: any = { value: formData.value };
      if (formData.ttl) {
        body.ttl = parseInt(formData.ttl);
      }

      const response = await fetch(`/api/kv/${encodeURIComponent(formData.key)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });

      if (response.ok) {
        setShowModal(false);
        setFormData({ key: '', value: '', ttl: '' });
        setEditingKey(null);
        await loadKeys();
        await loadStats();
      } else {
        setError('Failed to save key-value pair');
      }
    } catch (err) {
      setError('Failed to save key-value pair');
    }
  };

  const handleDelete = async (key: string) => {
    if (!confirm(`Delete key "${key}"?`)) return;
    
    try {
      const response = await fetch(`/api/kv/${encodeURIComponent(key)}`, {
        method: 'DELETE',
      });

      if (response.ok) {
        await loadKeys();
        await loadStats();
      } else {
        setError('Failed to delete key');
      }
    } catch (err) {
      setError('Failed to delete key');
    }
  };

  const handleEdit = (kv: KeyValuePair) => {
    setEditingKey(kv);
    setFormData({
      key: kv.key,
      value: kv.value,
      ttl: kv.ttl ? kv.ttl.toString() : '',
    });
    setShowModal(true);
  };

  const copyToClipboard = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const filteredKeys = keys.filter(kv =>
    searchQuery === '' || 
    kv.key.toLowerCase().includes(searchQuery.toLowerCase()) ||
    kv.value.toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">Key-Value Store</h2>
          <p className="text-sm text-gray-600 mt-1">Redis-style key-value storage with TTL support</p>
        </div>
        <button
          onClick={() => {
            setEditingKey(null);
            setFormData({ key: '', value: '', ttl: '' });
            setShowModal(true);
          }}
          className="flex items-center gap-2 px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors"
        >
          <Plus className="w-4 h-4" />
          Add Key
        </button>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
          {error}
        </div>
      )}

      {/* Stats Cards */}
      {stats && (
        <div className="grid grid-cols-4 gap-4">
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-gray-900">{stats.total_keys || 0}</div>
              <div className="text-sm text-gray-600">Total Keys</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-gray-900">{stats.memory_usage || '0 B'}</div>
              <div className="text-sm text-gray-600">Memory Usage</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-gray-900">{stats.expired_keys || 0}</div>
              <div className="text-sm text-gray-600">Expired Keys</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-gray-900">{stats.hit_rate || '0%'}</div>
              <div className="text-sm text-gray-600">Cache Hit Rate</div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Main Content */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Key-Value Pairs</CardTitle>
            <div className="flex items-center gap-2">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input
                  type="text"
                  placeholder="Search keys..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-10 pr-4 py-2 border rounded-lg text-sm w-64"
                />
              </div>
              <button
                onClick={loadKeys}
                disabled={loading}
                className="p-2 border rounded-lg hover:bg-gray-50 transition-colors"
              >
                <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
              </button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="text-center py-12 text-gray-500">
              <RefreshCw className="w-8 h-8 animate-spin mx-auto mb-2" />
              Loading keys...
            </div>
          ) : filteredKeys.length === 0 ? (
            <div className="text-center py-12 text-gray-500">
              <Key className="w-12 h-12 mx-auto mb-2 opacity-50" />
              <p>No keys found</p>
              <button
                onClick={() => setShowModal(true)}
                className="mt-4 text-mantis-600 hover:text-mantis-700"
              >
                Add your first key
              </button>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-3 px-4 text-sm font-medium text-gray-700">Key</th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-gray-700">Value</th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-gray-700">TTL</th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-gray-700">Updated</th>
                    <th className="text-right py-3 px-4 text-sm font-medium text-gray-700">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredKeys.map((kv) => (
                    <tr key={kv.key} className="border-b hover:bg-gray-50">
                      <td className="py-3 px-4">
                        <div className="flex items-center gap-2">
                          <Key className="w-4 h-4 text-mantis-600" />
                          <span className="font-mono text-sm">{kv.key}</span>
                          <button
                            onClick={() => copyToClipboard(kv.key, kv.key)}
                            className="p-1 hover:bg-gray-200 rounded"
                          >
                            {copiedKey === kv.key ? (
                              <Check className="w-3 h-3 text-green-600" />
                            ) : (
                              <Copy className="w-3 h-3 text-gray-400" />
                            )}
                          </button>
                        </div>
                      </td>
                      <td className="py-3 px-4">
                        <div className="max-w-md truncate font-mono text-sm text-gray-600">
                          {kv.value}
                        </div>
                      </td>
                      <td className="py-3 px-4">
                        {kv.ttl ? (
                          <div className="flex items-center gap-1 text-sm text-gray-600">
                            <Clock className="w-3 h-3" />
                            {kv.ttl}s
                          </div>
                        ) : (
                          <span className="text-sm text-gray-400">No expiry</span>
                        )}
                      </td>
                      <td className="py-3 px-4 text-sm text-gray-600">
                        {kv.updated_at ? new Date(kv.updated_at).toLocaleString() : '-'}
                      </td>
                      <td className="py-3 px-4">
                        <div className="flex items-center justify-end gap-2">
                          <button
                            onClick={() => handleEdit(kv)}
                            className="p-1 hover:bg-gray-100 rounded"
                          >
                            <Edit2 className="w-4 h-4 text-gray-600" />
                          </button>
                          <button
                            onClick={() => handleDelete(kv.key)}
                            className="p-1 hover:bg-red-50 rounded"
                          >
                            <Trash2 className="w-4 h-4 text-red-600" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Add/Edit Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 max-w-lg w-full mx-4">
            <h3 className="text-lg font-semibold mb-4">
              {editingKey ? 'Edit Key-Value Pair' : 'Add Key-Value Pair'}
            </h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Key</label>
                <input
                  type="text"
                  value={formData.key}
                  onChange={(e) => setFormData({ ...formData, key: e.target.value })}
                  disabled={!!editingKey}
                  className="w-full px-3 py-2 border rounded-lg font-mono text-sm"
                  placeholder="my:key:name"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Value</label>
                <textarea
                  value={formData.value}
                  onChange={(e) => setFormData({ ...formData, value: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg font-mono text-sm h-32"
                  placeholder="Value (can be JSON, text, etc.)"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  TTL (seconds) - Optional
                </label>
                <input
                  type="number"
                  value={formData.ttl}
                  onChange={(e) => setFormData({ ...formData, ttl: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm"
                  placeholder="Leave empty for no expiration"
                />
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => setShowModal(false)}
                className="px-4 py-2 border rounded-lg hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                className="px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700"
              >
                {editingKey ? 'Update' : 'Add'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
