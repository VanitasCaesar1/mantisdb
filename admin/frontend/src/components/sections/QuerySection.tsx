import { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';

interface QueryResult {
  columns: string[];
  rows: any[][];
  rowCount: number;
  executionTime: number;
}

interface QueryHistoryItem {
  id: string;
  query: string;
  timestamp: Date;
  executionTime: number;
  rowCount: number;
}

const QUERY_TEMPLATES = [
  {
    name: 'Select All',
    query: 'SELECT * FROM table_name LIMIT 100;',
    description: 'Retrieve all columns from a table'
  },
  {
    name: 'Insert Row',
    query: `INSERT INTO table_name (column1, column2, column3)\nVALUES ('value1', 'value2', 'value3');`,
    description: 'Insert a new row'
  },
  {
    name: 'Update Row',
    query: `UPDATE table_name\nSET column1 = 'new_value'\nWHERE id = 1;`,
    description: 'Update existing rows'
  },
  {
    name: 'Delete Row',
    query: 'DELETE FROM table_name WHERE id = 1;',
    description: 'Delete rows'
  },
  {
    name: 'Create Table',
    query: `CREATE TABLE table_name (\n  id INTEGER PRIMARY KEY,\n  name TEXT NOT NULL,\n  email TEXT UNIQUE,\n  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP\n);`,
    description: 'Create a new table'
  },
  {
    name: 'Join Tables',
    query: `SELECT u.*, o.order_id, o.total\nFROM users u\nINNER JOIN orders o ON u.id = o.user_id\nWHERE u.active = true;`,
    description: 'Join multiple tables'
  },
  {
    name: 'Aggregate',
    query: `SELECT\n  category,\n  COUNT(*) as count,\n  AVG(price) as avg_price,\n  SUM(quantity) as total_qty\nFROM products\nGROUP BY category\nORDER BY count DESC;`,
    description: 'Use aggregate functions'
  },
  {
    name: 'Subquery',
    query: `SELECT *\nFROM users\nWHERE id IN (\n  SELECT user_id\n  FROM orders\n  WHERE total > 1000\n);`,
    description: 'Use subqueries'
  }
];

export function QuerySection() {
  const [query, setQuery] = useState('SELECT * FROM users LIMIT 10;');
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [executing, setExecuting] = useState(false);
  const [history, setHistory] = useState<QueryHistoryItem[]>([]);
  const [showTemplates, setShowTemplates] = useState(false);

  const executeQuery = async () => {
    if (!query.trim()) return;

    setExecuting(true);
    setError(null);
    const startTime = Date.now();

    try {
      const response = await fetch('/api/query', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: query.trim() })
      });

      const data = await response.json();
      const executionTime = Date.now() - startTime;

      if (!response.ok) {
        throw new Error(data.error || 'Query execution failed');
      }

      setResult({
        columns: data.columns || [],
        rows: data.rows || [],
        rowCount: data.row_count || 0,
        executionTime
      });

      // Add to history
      setHistory([
        {
          id: Date.now().toString(),
          query: query.trim(),
          timestamp: new Date(),
          executionTime,
          rowCount: data.row_count || 0
        },
        ...history.slice(0, 9) // Keep last 10
      ]);
    } catch (err: any) {
      setError(err.message || 'Failed to execute query');
      setResult(null);
    } finally {
      setExecuting(false);
    }
  };

  const loadHistoryQuery = (historyQuery: string) => {
    setQuery(historyQuery);
  };

  const loadTemplate = (template: string) => {
    setQuery(template);
    setShowTemplates(false);
  };

  // Keyboard shortcut for execution
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
        e.preventDefault();
        if (query.trim() && !executing) {
          executeQuery();
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [query, executing]);

  return (
    <div className="space-y-6">
      {/* SQL Editor */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>SQL Editor</CardTitle>
            <Button variant="secondary" onClick={() => setShowTemplates(!showTemplates)}>
              {showTemplates ? 'Hide Templates' : 'Show Templates'}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {/* Query Templates */}
            {showTemplates && (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-2 mb-4">
                {QUERY_TEMPLATES.map((template, index) => (
                  <button
                    key={index}
                    onClick={() => loadTemplate(template.query)}
                    className="p-3 text-left border border-gray-200 rounded-lg hover:border-mantis-400 hover:bg-mantis-50 transition-colors"
                    title={template.description}
                  >
                    <div className="text-sm font-medium text-gray-900">{template.name}</div>
                    <div className="text-xs text-gray-500 mt-1">{template.description}</div>
                  </button>
                ))}
              </div>
            )}

            {/* Editor */}
            <div className="relative">
              <textarea
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="w-full h-64 p-4 font-mono text-sm border border-gray-300 rounded-lg focus:ring-2 focus:ring-mantis-500 focus:border-transparent bg-gray-50"
                placeholder="Enter your SQL query here..."
                spellCheck={false}
              />
              <div className="absolute bottom-2 right-2 text-xs text-gray-400 bg-white px-2 py-1 rounded">
                {query.split('\n').length} lines • {query.length} chars
              </div>
            </div>

            {/* Actions */}
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-4">
                <div className="text-sm text-gray-600">
                  <kbd className="px-2 py-1 bg-gray-100 border border-gray-300 rounded text-xs">Cmd</kbd> + 
                  <kbd className="px-2 py-1 bg-gray-100 border border-gray-300 rounded text-xs ml-1">Enter</kbd> to execute
                </div>
              </div>
              <div className="flex space-x-2">
                <Button
                  variant="secondary"
                  onClick={() => setQuery('')}
                  disabled={executing}
                >
                  Clear
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => {
                    const formatted = query.trim().split('\n').map(line => line.trim()).join('\n');
                    setQuery(formatted);
                  }}
                  disabled={executing}
                >
                  Format
                </Button>
                <Button
                  onClick={executeQuery}
                  disabled={executing || !query.trim()}
                >
                  {executing ? 'Executing...' : '▶ Execute Query'}
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Error Display */}
      {error && (
        <Card>
          <CardContent className="p-6">
            <div className="flex items-start space-x-3">
              <div className="flex-shrink-0">
                <svg className="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
                  <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
                </svg>
              </div>
              <div className="flex-1">
                <h3 className="text-sm font-medium text-red-800">Query Error</h3>
                <p className="mt-1 text-sm text-red-700">{error}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Results */}
      {result && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Query Results</CardTitle>
              <div className="text-sm text-gray-600">
                {result.rowCount} rows in {result.executionTime}ms
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    {result.columns.map((column, index) => (
                      <th
                        key={index}
                        className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                      >
                        {column}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {result.rows.map((row, rowIndex) => (
                    <tr key={rowIndex} className="hover:bg-gray-50">
                      {row.map((cell, cellIndex) => (
                        <td
                          key={cellIndex}
                          className="px-6 py-4 whitespace-nowrap text-sm text-gray-900"
                        >
                          {cell === null ? <span className="text-gray-400">NULL</span> : String(cell)}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Query History */}
      {history.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Query History</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {history.map((item) => (
                <div
                  key={item.id}
                  className="p-3 border border-gray-200 rounded-lg hover:border-mantis-300 cursor-pointer transition-colors"
                  onClick={() => loadHistoryQuery(item.query)}
                >
                  <div className="flex items-start justify-between mb-2">
                    <code className="text-sm text-gray-900 flex-1">{item.query}</code>
                    <span className="text-xs text-gray-500 ml-4">
                      {item.timestamp.toLocaleTimeString()}
                    </span>
                  </div>
                  <div className="flex items-center space-x-4 text-xs text-gray-600">
                    <span>{item.rowCount} rows</span>
                    <span>{item.executionTime}ms</span>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
