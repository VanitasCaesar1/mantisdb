import React, { useState, useRef } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';
import type { QueryResult, QueryHistory } from '../../types';

export interface SQLEditorProps {
  onExecuteQuery: (query: string) => Promise<QueryResult>;
  loading?: boolean;
  history?: QueryHistory[];
}

const SQLEditor: React.FC<SQLEditorProps> = ({
  onExecuteQuery,
  loading = false
}) => {
  const [query, setQuery] = useState('');
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Sample queries for quick access
  const sampleQueries = [
    'SELECT * FROM users LIMIT 10;',
    'SHOW TABLES;',
    'DESCRIBE users;',
    'SELECT COUNT(*) FROM users;',
    'SELECT * FROM users WHERE created_at > NOW() - INTERVAL 1 DAY;'
  ];

  const handleExecute = async () => {
    if (!query.trim()) return;

    setError(null);
    setResult(null);

    try {
      const queryResult = await onExecuteQuery(query.trim());
      setResult(queryResult);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Query execution failed');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    // Execute query with Ctrl+Enter or Cmd+Enter
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      handleExecute();
    }

    // Handle tab indentation
    if (e.key === 'Tab') {
      e.preventDefault();
      const textarea = textareaRef.current;
      if (textarea) {
        const start = textarea.selectionStart;
        const end = textarea.selectionEnd;
        const newQuery = query.substring(0, start) + '  ' + query.substring(end);
        setQuery(newQuery);
        
        // Set cursor position after the inserted spaces
        setTimeout(() => {
          textarea.selectionStart = textarea.selectionEnd = start + 2;
        }, 0);
      }
    }
  };

  const insertSampleQuery = (sampleQuery: string) => {
    setQuery(sampleQuery);
    textareaRef.current?.focus();
  };

  const formatQuery = () => {
    // Basic SQL formatting
    const formatted = query
      .replace(/\s+/g, ' ')
      .replace(/\s*,\s*/g, ',\n  ')
      .replace(/\s+(FROM|WHERE|GROUP BY|ORDER BY|HAVING|LIMIT|JOIN|LEFT JOIN|RIGHT JOIN|INNER JOIN)\s+/gi, '\n$1 ')
      .replace(/\s*;\s*/g, ';\n')
      .trim();
    
    setQuery(formatted);
  };

  const clearQuery = () => {
    setQuery('');
    setResult(null);
    setError(null);
    textareaRef.current?.focus();
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>SQL Editor</CardTitle>
            <div className="flex items-center space-x-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={formatQuery}
                disabled={!query.trim()}
              >
                Format
              </Button>
              <Button
                variant="secondary"
                size="sm"
                onClick={clearQuery}
              >
                Clear
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={handleExecute}
                loading={loading}
                disabled={!query.trim()}
              >
                Execute (Ctrl+Enter)
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {/* Sample Queries */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Quick Start
              </label>
              <div className="flex flex-wrap gap-2">
                {sampleQueries.map((sample, index) => (
                  <button
                    key={index}
                    onClick={() => insertSampleQuery(sample)}
                    className="px-3 py-1 text-xs bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-md transition-colors"
                  >
                    {sample.split(' ').slice(0, 3).join(' ')}...
                  </button>
                ))}
              </div>
            </div>

            {/* SQL Editor */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                SQL Query
              </label>
              <div className="relative">
                <textarea
                  ref={textareaRef}
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Enter your SQL query here..."
                  className="w-full h-48 px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-mantis-500 focus:border-mantis-500 font-mono text-sm resize-none"
                  spellCheck={false}
                />
                <div className="absolute bottom-2 right-2 text-xs text-gray-400">
                  Lines: {query.split('\n').length} | Chars: {query.length}
                </div>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Query Result */}
      {(result || error) && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>
                {error ? 'Query Error' : 'Query Result'}
              </CardTitle>
              {result && (
                <div className="text-sm text-gray-600">
                  {result.rowCount} rows in {result.executionTime}ms
                </div>
              )}
            </div>
          </CardHeader>
          <CardContent>
            {error ? (
              <div className="p-4 bg-red-50 border border-red-200 rounded-md">
                <div className="flex">
                  <div className="flex-shrink-0">
                    <svg className="h-5 w-5 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                  </div>
                  <div className="ml-3">
                    <h3 className="text-sm font-medium text-red-800">
                      Query execution failed
                    </h3>
                    <div className="mt-2 text-sm text-red-700">
                      <pre className="whitespace-pre-wrap font-mono">{error}</pre>
                    </div>
                  </div>
                </div>
              </div>
            ) : result && (
              <div className="space-y-4">
                {result.rows.length === 0 ? (
                  <div className="text-center py-8 text-gray-500">
                    Query executed successfully but returned no rows.
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="min-w-full divide-y divide-gray-200">
                      <thead className="bg-gray-50">
                        <tr>
                          {result.columns.map((column, index) => (
                            <th
                              key={index}
                              className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                            >
                              {column}
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody className="bg-white divide-y divide-gray-200">
                        {result.rows.slice(0, 100).map((row, rowIndex) => (
                          <tr key={rowIndex} className="hover:bg-gray-50">
                            {row.map((cell, cellIndex) => (
                              <td
                                key={cellIndex}
                                className="px-4 py-3 text-sm text-gray-900 max-w-xs truncate"
                                title={String(cell)}
                              >
                                {cell === null || cell === undefined ? (
                                  <span className="text-gray-400 italic">NULL</span>
                                ) : typeof cell === 'boolean' ? (
                                  <span className={`px-2 py-1 text-xs rounded-full ${
                                    cell ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                                  }`}>
                                    {cell ? 'true' : 'false'}
                                  </span>
                                ) : (
                                  String(cell)
                                )}
                              </td>
                            ))}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                    {result.rows.length > 100 && (
                      <div className="mt-4 text-center text-sm text-gray-500">
                        Showing first 100 rows of {result.rowCount} total rows
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
};

export default SQLEditor;