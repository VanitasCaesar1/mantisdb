import React, { useState, useEffect, useRef } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Input, Badge } from '../ui';
import { SearchIcon, RefreshIcon } from '../icons';
import { formatRelativeTime, debounce } from '../../utils';
import type { LogEntry } from '../../types';

export interface LogViewerProps {
  logs: LogEntry[];
  loading?: boolean;
  onRefresh: () => void;
  onLoadMore?: () => void;
  hasMore?: boolean;
  realTime?: boolean;
  onToggleRealTime?: (enabled: boolean) => void;
}

const LogViewer: React.FC<LogViewerProps> = ({
  logs,
  loading = false,
  onRefresh,
  onLoadMore,
  hasMore = false,
  realTime = false,
  onToggleRealTime
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [levelFilter, setLevelFilter] = useState<string>('');
  const [componentFilter, setComponentFilter] = useState<string>('');
  const [autoScroll, setAutoScroll] = useState(true);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const logsContainerRef = useRef<HTMLDivElement>(null);

  const debouncedSearch = debounce((term: string) => {
    // In a real implementation, this would trigger a server-side search
    console.log('Searching for:', term);
  }, 300);

  useEffect(() => {
    debouncedSearch(searchTerm);
  }, [searchTerm, debouncedSearch]);

  useEffect(() => {
    if (autoScroll && realTime) {
      logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, autoScroll, realTime]);

  const handleScroll = () => {
    if (!logsContainerRef.current) return;
    
    const { scrollTop, scrollHeight, clientHeight } = logsContainerRef.current;
    const isAtBottom = scrollHeight - scrollTop <= clientHeight + 100;
    
    if (autoScroll && !isAtBottom) {
      setAutoScroll(false);
    } else if (!autoScroll && isAtBottom) {
      setAutoScroll(true);
    }
  };

  const filteredLogs = logs.filter(log => {
    const matchesSearch = !searchTerm || 
      log.message.toLowerCase().includes(searchTerm.toLowerCase()) ||
      log.component.toLowerCase().includes(searchTerm.toLowerCase()) ||
      (log.request_id && log.request_id.toLowerCase().includes(searchTerm.toLowerCase()));
    
    const matchesLevel = !levelFilter || log.level === levelFilter;
    const matchesComponent = !componentFilter || log.component === componentFilter;
    
    return matchesSearch && matchesLevel && matchesComponent;
  });

  const getLevelColor = (level: string) => {
    switch (level) {
      case 'DEBUG': return 'bg-gray-100 text-gray-800';
      case 'INFO': return 'bg-blue-100 text-blue-800';
      case 'WARN': return 'bg-yellow-100 text-yellow-800';
      case 'ERROR': return 'bg-red-100 text-red-800';
      case 'FATAL': return 'bg-red-200 text-red-900';
      default: return 'bg-gray-100 text-gray-800';
    }
  };

  const getLevelIcon = (level: string) => {
    switch (level) {
      case 'DEBUG':
        return (
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      case 'INFO':
        return (
          <svg className="w-4 h-4 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      case 'WARN':
        return (
          <svg className="w-4 h-4 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.732-.833-2.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z" />
          </svg>
        );
      case 'ERROR':
      case 'FATAL':
        return (
          <svg className="w-4 h-4 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      default:
        return null;
    }
  };

  const uniqueComponents = Array.from(new Set(logs.map(log => log.component))).sort();
  const logLevels = ['DEBUG', 'INFO', 'WARN', 'ERROR', 'FATAL'];

  return (
    <Card className="h-full flex flex-col">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>System Logs</CardTitle>
            <p className="text-sm text-gray-600 mt-1">
              {filteredLogs.length} of {logs.length} log entries
            </p>
          </div>
          <div className="flex items-center space-x-3">
            {onToggleRealTime && (
              <label className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  checked={realTime}
                  onChange={(e) => onToggleRealTime(e.target.checked)}
                  className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
                />
                <span className="text-sm text-gray-700">Real-time</span>
              </label>
            )}
            <label className="flex items-center space-x-2">
              <input
                type="checkbox"
                checked={autoScroll}
                onChange={(e) => setAutoScroll(e.target.checked)}
                className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
              />
              <span className="text-sm text-gray-700">Auto-scroll</span>
            </label>
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
      <CardContent className="flex-1 flex flex-col">
        {/* Filters */}
        <div className="flex items-center space-x-4 mb-4">
          <div className="flex-1">
            <Input
              placeholder="Search logs..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              leftIcon={<SearchIcon className="w-4 h-4" />}
            />
          </div>
          <div className="flex items-center space-x-2">
            <select
              value={levelFilter}
              onChange={(e) => setLevelFilter(e.target.value)}
              className="border border-gray-300 rounded px-3 py-2 text-sm"
            >
              <option value="">All Levels</option>
              {logLevels.map(level => (
                <option key={level} value={level}>{level}</option>
              ))}
            </select>
            <select
              value={componentFilter}
              onChange={(e) => setComponentFilter(e.target.value)}
              className="border border-gray-300 rounded px-3 py-2 text-sm"
            >
              <option value="">All Components</option>
              {uniqueComponents.map(component => (
                <option key={component} value={component}>{component}</option>
              ))}
            </select>
          </div>
        </div>

        {/* Logs Container */}
        <div 
          ref={logsContainerRef}
          onScroll={handleScroll}
          className="flex-1 overflow-y-auto bg-gray-50 rounded-lg p-4 font-mono text-sm"
        >
          {loading && logs.length === 0 ? (
            <div className="text-center py-8">
              <div className="animate-spin w-8 h-8 border-2 border-mantis-600 border-t-transparent rounded-full mx-auto mb-4"></div>
              <p className="text-gray-600">Loading logs...</p>
            </div>
          ) : filteredLogs.length === 0 ? (
            <div className="text-center py-8">
              <div className="text-gray-400 mb-4">
                <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
              </div>
              <h3 className="text-lg font-medium text-gray-900 mb-2">No logs found</h3>
              <p className="text-gray-600">
                {searchTerm || levelFilter || componentFilter
                  ? 'No logs match your search criteria.'
                  : 'No log entries available.'
                }
              </p>
            </div>
          ) : (
            <div className="space-y-2">
              {filteredLogs.map((log) => (
                <div
                  key={log.id}
                  className={`p-3 rounded border-l-4 ${
                    log.level === 'ERROR' || log.level === 'FATAL'
                      ? 'bg-red-50 border-red-400'
                      : log.level === 'WARN'
                      ? 'bg-yellow-50 border-yellow-400'
                      : log.level === 'INFO'
                      ? 'bg-blue-50 border-blue-400'
                      : 'bg-white border-gray-300'
                  }`}
                >
                  <div className="flex items-start space-x-3">
                    <div className="flex-shrink-0 mt-0.5">
                      {getLevelIcon(log.level)}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center space-x-2 mb-1">
                        <Badge className={getLevelColor(log.level)} size="sm">
                          {log.level}
                        </Badge>
                        <span className="text-xs text-gray-500">
                          {formatRelativeTime(log.timestamp)}
                        </span>
                        <Badge variant="default" size="sm">
                          {log.component}
                        </Badge>
                        {log.request_id && (
                          <Badge variant="info" size="sm">
                            {log.request_id}
                          </Badge>
                        )}
                      </div>
                      <p className="text-gray-900 break-words">
                        {log.message}
                      </p>
                      {log.metadata && Object.keys(log.metadata).length > 0 && (
                        <details className="mt-2">
                          <summary className="text-xs text-gray-500 cursor-pointer hover:text-gray-700">
                            Show metadata
                          </summary>
                          <pre className="mt-1 text-xs text-gray-600 bg-gray-100 p-2 rounded overflow-x-auto">
                            {JSON.stringify(log.metadata, null, 2)}
                          </pre>
                        </details>
                      )}
                    </div>
                  </div>
                </div>
              ))}
              
              {hasMore && onLoadMore && (
                <div className="text-center py-4">
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={onLoadMore}
                    loading={loading}
                  >
                    Load More Logs
                  </Button>
                </div>
              )}
              
              <div ref={logsEndRef} />
            </div>
          )}
        </div>

        {/* Status Bar */}
        <div className="flex items-center justify-between mt-4 pt-4 border-t border-gray-200 text-sm text-gray-600">
          <div className="flex items-center space-x-4">
            <span>Total: {logs.length} entries</span>
            <span>Filtered: {filteredLogs.length} entries</span>
            {realTime && (
              <div className="flex items-center space-x-1">
                <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></div>
                <span>Live</span>
              </div>
            )}
          </div>
          <div className="flex items-center space-x-2">
            {autoScroll && (
              <span className="text-green-600">Auto-scroll enabled</span>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
};

export default LogViewer;