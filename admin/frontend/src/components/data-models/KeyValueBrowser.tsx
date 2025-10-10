import React, { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';

interface KVEntry {
  key: string;
  value: any;
  ttl?: number;
  created_at: number;
  updated_at: number;
  version: number;
  metadata: {
    content_type: string;
    tags: string[];
  };
}

export const KeyValueBrowser: React.FC = () => {
  const [keys, setKeys] = useState<string[]>([]);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [selectedEntry, setSelectedEntry] = useState<KVEntry | null>(null);
  const [loading, setLoading] = useState(false);
  const [searchPrefix, setSearchPrefix] = useState('');
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');
  const [newTTL, setNewTTL] = useState('');
  const [showAddModal, setShowAddModal] = useState(false);

  useEffect(() => {
    loadKeys();
  }, [searchPrefix]);

  const loadKeys = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (searchPrefix) params.append('prefix', searchPrefix);
      params.append('limit', '100');
      
      const response = await fetch(`http://localhost:8081/api/kv/query?${params}`);
      const data = await response.json();
      
      if (data.keys) {
        setKeys(data.keys);
      }
    } catch (error) {
      console.error('Failed to load keys:', error);
    } finally {
      setLoading(false);
    }
  };

  const loadEntry = async (key: string) => {
    try {
      const response = await fetch(`http://localhost:8081/api/kv/${encodeURIComponent(key)}`);
      const data = await response.json();
      
      if (data.success && data.data) {
        setSelectedEntry(data.data);
        setSelectedKey(key);
      }
    } catch (error) {
      console.error('Failed to load entry:', error);
    }
  };

  const addEntry = async () => {
    try {
      let value;
      try {
        value = JSON.parse(newValue);
      } catch {
        value = newValue;
      }

      const body: any = { value };
      if (newTTL) {
        body.ttl = parseInt(newTTL);
      }

      const response = await fetch(`http://localhost:8081/api/kv/${encodeURIComponent(newKey)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });

      const data = await response.json();
      
      if (data.success) {
        setShowAddModal(false);
        setNewKey('');
        setNewValue('');
        setNewTTL('');
        loadKeys();
        alert('Entry added successfully!');
      }
    } catch (error) {
      console.error('Failed to add entry:', error);
      alert('Failed to add entry');
    }
  };

  const deleteEntry = async (key: string) => {
    if (!confirm(`Delete key "${key}"?`)) return;

    try {
      const response = await fetch(`http://localhost:8081/api/kv/${encodeURIComponent(key)}`, {
        method: 'DELETE',
      });

      const data = await response.json();
      
      if (data.success) {
        setSelectedKey(null);
        setSelectedEntry(null);
        loadKeys();
        alert('Entry deleted successfully!');
      }
    } catch (error) {
      console.error('Failed to delete entry:', error);
      alert('Failed to delete entry');
    }
  };

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
      {/* Keys List */}
      <Card className="lg:col-span-1">
        <CardHeader>
          <CardTitle>Keys</CardTitle>
          <div className="mt-2 space-y-2">
            <input
              type="text"
              placeholder="Search by prefix..."
              value={searchPrefix}
              onChange={(e) => setSearchPrefix(e.target.value)}
              className="w-full px-3 py-2 border rounded"
            />
            <Button onClick={() => setShowAddModal(true)} className="w-full">
              Add New Key
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="text-center py-4">Loading...</div>
          ) : keys.length === 0 ? (
            <div className="text-center py-4 text-gray-500">No keys found</div>
          ) : (
            <div className="space-y-1 max-h-96 overflow-y-auto">
              {keys.map(key => (
                <div
                  key={key}
                  onClick={() => loadEntry(key)}
                  className={`p-2 rounded cursor-pointer hover:bg-gray-100 ${
                    selectedKey === key ? 'bg-mantis-100 text-mantis-800' : ''
                  }`}
                >
                  <div className="font-mono text-sm truncate">{key}</div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Entry Details */}
      <Card className="lg:col-span-2">
        <CardHeader>
          <div className="flex justify-between items-center">
            <CardTitle>
              {selectedKey ? `Key: ${selectedKey}` : 'Select a key'}
            </CardTitle>
            {selectedKey && (
              <Button variant="secondary" onClick={() => deleteEntry(selectedKey)}>
                Delete
              </Button>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {selectedEntry ? (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Value</label>
                <pre className="p-4 bg-gray-50 rounded border overflow-x-auto">
                  {JSON.stringify(selectedEntry.value, null, 2)}
                </pre>
              </div>
              
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Version</label>
                  <div className="text-sm">{selectedEntry.version}</div>
                </div>
                
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">TTL</label>
                  <div className="text-sm">{selectedEntry.ttl ? `${selectedEntry.ttl}s` : 'No expiration'}</div>
                </div>
                
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Created At</label>
                  <div className="text-sm">{new Date(selectedEntry.created_at * 1000).toLocaleString()}</div>
                </div>
                
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Updated At</label>
                  <div className="text-sm">{new Date(selectedEntry.updated_at * 1000).toLocaleString()}</div>
                </div>
              </div>
              
              {selectedEntry.metadata.tags.length > 0 && (
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Tags</label>
                  <div className="flex flex-wrap gap-2">
                    {selectedEntry.metadata.tags.map(tag => (
                      <span key={tag} className="px-2 py-1 bg-blue-100 text-blue-800 rounded text-sm">
                        {tag}
                      </span>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div className="text-center py-12 text-gray-500">
              Select a key to view details
            </div>
          )}
        </CardContent>
      </Card>

      {/* Add Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card className="w-full max-w-lg">
            <CardHeader>
              <CardTitle>Add New Key-Value Pair</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Key</label>
                  <input
                    type="text"
                    value={newKey}
                    onChange={(e) => setNewKey(e.target.value)}
                    className="w-full px-3 py-2 border rounded"
                    placeholder="my-key"
                  />
                </div>
                
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Value (JSON or text)</label>
                  <textarea
                    value={newValue}
                    onChange={(e) => setNewValue(e.target.value)}
                    className="w-full px-3 py-2 border rounded font-mono"
                    rows={6}
                    placeholder='{"name": "value"}'
                  />
                </div>
                
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">TTL (seconds, optional)</label>
                  <input
                    type="number"
                    value={newTTL}
                    onChange={(e) => setNewTTL(e.target.value)}
                    className="w-full px-3 py-2 border rounded"
                    placeholder="3600"
                  />
                </div>
                
                <div className="flex gap-2">
                  <Button onClick={addEntry} disabled={!newKey || !newValue}>
                    Add
                  </Button>
                  <Button variant="secondary" onClick={() => setShowAddModal(false)}>
                    Cancel
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
};
