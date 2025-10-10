import { useState, useEffect, useRef } from 'react';
import type React from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Input } from '../ui';
import { LogsIcon } from '../icons';
import { apiClient } from '../../api/client';

interface LogEntry {
  timestamp: string;
  level: 'debug' | 'info' | 'warn' | 'error';
  message: string;
  component?: string;
}

export function LogsSection() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [filter, setFilter] = useState('');
  const [levelFilter, setLevelFilter] = useState<string>('all');
  const [live, setLive] = useState(true);
  const [loading, setLoading] = useState(false);
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    fetchLogs();
  }, []);

  useEffect(() => {
    let interval: number | undefined;
    if (live) {
      // Open SSE stream
      apiClient.createLogsWebSocket((data: any) => {
        // Data could be either {type: 'log_entry', data: {...}} or the entry itself
        const entry = data?.type === 'log_entry' ? data.data : data;
        if (!entry) return;
        const normalized: LogEntry = {
          timestamp: typeof entry.timestamp === 'string' ? entry.timestamp : new Date(entry.timestamp).toISOString(),
          level: (entry.level || 'info').toLowerCase(),
          message: entry.message || '',
          component: entry.component || undefined,
        };
        setLogs(prev => [normalized, ...prev].slice(0, 1000));
      }).then((es) => {
        esRef.current = es;
      });
    } else {
      // Poll every 5s when live is off
      interval = window.setInterval(fetchLogs, 5000);
    }
    return () => {
      if (interval) window.clearInterval(interval);
      if (esRef.current) { esRef.current.close(); esRef.current = null; }
    };
  }, [live]);

  const fetchLogs = async () => {
    try {
      setLoading(true);
      const resp = await apiClient.getLogs({ limit: 200 });
      if (resp.success) {
        const raw = (resp.data as any)?.logs || [];
        const normalized = raw.map((l: any) => ({
          timestamp: typeof l.timestamp === 'string' ? l.timestamp : new Date(l.timestamp).toISOString(),
          level: (l.level || 'info').toLowerCase(),
          message: l.message || '',
          component: l.component || undefined,
        }));
        setLogs(normalized);
      }
    } catch (err) {
      console.error('Failed to fetch logs:', err);
    } finally {
      setLoading(false);
    }
  };

  const filteredLogs = logs.filter(log => {
    const matchesLevel = levelFilter === 'all' || log.level === levelFilter;
    const matchesSearch = !filter || 
      log.message.toLowerCase().includes(filter.toLowerCase()) ||
      log.component?.toLowerCase().includes(filter.toLowerCase());
    return matchesLevel && matchesSearch;
  });

  const getLevelColor = (level: string) => {
    switch (level) {
      case 'error': return 'text-red-600 bg-red-50';
      case 'warn': return 'text-yellow-600 bg-yellow-50';
      case 'info': return 'text-blue-600 bg-blue-50';
      case 'debug': return 'text-gray-600 bg-gray-50';
      default: return 'text-gray-600 bg-gray-50';
    }
  };

  return (
    <div className="space-y-6">
      {/* Controls */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center space-x-4">
            <div className="flex-1">
              <Input
                type="text"
                placeholder="Search logs..."
                value={filter}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setFilter(e.target.value)}
              />
            </div>
            <select
              value={levelFilter}
              onChange={(e: React.ChangeEvent<HTMLSelectElement>) => setLevelFilter(e.target.value)}
              className="px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-mantis-500 focus:border-transparent"
            >
              <option value="all">All Levels</option>
              <option value="debug">Debug</option>
              <option value="info">Info</option>
              <option value="warn">Warning</option>
              <option value="error">Error</option>
            </select>
            <Button
              variant="secondary"
              onClick={() => setLive(!live)}
            >
              {live ? 'Live: ON' : 'Live: OFF'}
            </Button>
            <Button variant="secondary" onClick={fetchLogs} disabled={loading}>
              {loading ? 'Refreshing...' : 'Refresh'}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Logs Display */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>System Logs</CardTitle>
            <span className="text-sm text-gray-600">
              {filteredLogs.length} entries
            </span>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-1 max-h-[600px] overflow-y-auto font-mono text-sm">
            {filteredLogs.length === 0 ? (
              <div className="text-center py-12">
                <LogsIcon className="w-12 h-12 mx-auto text-gray-400 mb-4" />
                <p className="text-gray-600">No logs found</p>
              </div>
            ) : (
              filteredLogs.map((log, index) => (
                <div
                  key={index}
                  className={`p-3 rounded ${getLevelColor(log.level)} hover:opacity-80 transition-opacity`}
                >
                  <div className="flex items-start space-x-3">
                    <span className="text-xs text-gray-500 whitespace-nowrap">
                      {new Date(log.timestamp).toLocaleTimeString()}
                    </span>
                    <span className={`text-xs font-medium uppercase px-2 py-0.5 rounded ${getLevelColor(log.level)}`}>
                      {log.level}
                    </span>
                    {log.component && (
                      <span className="text-xs text-gray-500">
                        [{log.component}]
                      </span>
                    )}
                    <span className="flex-1 text-gray-900">
                      {log.message}
                    </span>
                  </div>
                </div>
              ))
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
