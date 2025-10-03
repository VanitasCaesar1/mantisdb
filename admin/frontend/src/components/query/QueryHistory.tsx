import React, { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Input, Badge } from '../ui';
import { SearchIcon, RefreshIcon } from '../icons';
import { formatRelativeTime, formatDuration, truncate } from '../../utils';
import type { QueryHistory as QueryHistoryType } from '../../types';

export interface QueryHistoryProps {
  history: QueryHistoryType[];
  loading?: boolean;
  onRefresh: () => void;
  onSelectQuery: (query: string) => void;
  onClearHistory?: () => void;
}

const QueryHistory: React.FC<QueryHistoryProps> = ({
  history,
  loading = false,
  onRefresh,
  onSelectQuery,
  onClearHistory
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'success' | 'error'>('all');

  const filteredHistory = history.filter(item => {
    const matchesSearch = item.query.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesStatus = statusFilter === 'all' || item.status === statusFilter;
    return matchesSearch && matchesStatus;
  });

  const getStatusColor = (status: 'success' | 'error') => {
    return status === 'success' ? 'success' : 'danger';
  };

  const formatQuery = (query: string) => {
    return query
      .replace(/\s+/g, ' ')
      .trim();
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>Query History</CardTitle>
            <p className="text-sm text-gray-600 mt-1">
              {filteredHistory.length} of {history.length} queries
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
            {onClearHistory && (
              <Button
                variant="danger"
                size="sm"
                onClick={onClearHistory}
                disabled={history.length === 0}
              >
                Clear History
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {/* Filters */}
          <div className="flex items-center space-x-4">
            <div className="flex-1">
              <Input
                placeholder="Search queries..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                leftIcon={<SearchIcon className="w-4 h-4" />}
              />
            </div>
            <div className="flex items-center space-x-2">
              <span className="text-sm text-gray-700">Status:</span>
              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value as 'all' | 'success' | 'error')}
                className="border border-gray-300 rounded px-3 py-1 text-sm"
              >
                <option value="all">All</option>
                <option value="success">Success</option>
                <option value="error">Error</option>
              </select>
            </div>
          </div>

          {/* History List */}
          {loading ? (
            <div className="text-center py-8">
              <div className="animate-spin w-8 h-8 border-2 border-mantis-600 border-t-transparent rounded-full mx-auto mb-4"></div>
              <p className="text-gray-600">Loading query history...</p>
            </div>
          ) : filteredHistory.length === 0 ? (
            <div className="text-center py-8">
              <div className="text-gray-400 mb-4">
                <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
              </div>
              <h3 className="text-lg font-medium text-gray-900 mb-2">No queries found</h3>
              <p className="text-gray-600">
                {searchTerm || statusFilter !== 'all' 
                  ? 'No queries match your search criteria.' 
                  : 'Execute your first query to see it here.'
                }
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {filteredHistory.map((item) => (
                <div
                  key={item.id}
                  className="border border-gray-200 rounded-lg p-4 hover:bg-gray-50 transition-colors"
                >
                  <div className="flex items-start justify-between">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center space-x-2 mb-2">
                        <Badge variant={getStatusColor(item.status)} size="sm">
                          {item.status}
                        </Badge>
                        <span className="text-sm text-gray-500">
                          {formatRelativeTime(item.timestamp)}
                        </span>
                        <span className="text-sm text-gray-500">
                          {formatDuration(item.executionTime)}
                        </span>
                        {item.status === 'success' && (
                          <span className="text-sm text-gray-500">
                            {item.rowCount} rows
                          </span>
                        )}
                      </div>
                      
                      <div className="mb-3">
                        <code className="text-sm bg-gray-100 p-2 rounded block overflow-x-auto">
                          {truncate(formatQuery(item.query), 200)}
                        </code>
                      </div>

                      {item.error && (
                        <div className="mb-3 p-2 bg-red-50 border border-red-200 rounded text-sm text-red-700">
                          {truncate(item.error, 150)}
                        </div>
                      )}
                    </div>

                    <div className="flex items-center space-x-2 ml-4">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => onSelectQuery(item.query)}
                      >
                        Use Query
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
  );
};

export default QueryHistory;