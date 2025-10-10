import React, { useState, useRef } from 'react';
import Editor from '@monaco-editor/react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';

interface QueryResult {
  success: boolean;
  data?: any;
  error?: string;
  duration_ms?: number;
  rows_affected?: number;
}

interface QueryHistoryItem {
  id: string;
  query: string;
  timestamp: Date;
  duration_ms: number;
  success: boolean;
}

export const SQLEditor: React.FC = () => {
  const [query, setQuery] = useState('SELECT * FROM users LIMIT 10;');
  const [results, setResults] = useState<QueryResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [history, setHistory] = useState<QueryHistoryItem[]>([]);
  const [activeTab, setActiveTab] = useState<'results' | 'history'>('results');
  const editorRef = useRef<any>(null);

  const handleEditorMount = (editor: any) => {
    editorRef.current = editor;
    
    // Add custom keybindings
    editor.addAction({
      id: 'execute-query',
      label: 'Execute Query',
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter],
      run: () => executeQuery(),
    });
  };

  const executeQuery = async () => {
    const queryText = editorRef.current?.getValue() || query;
    
    if (!queryText.trim()) {
      alert('Please enter a query');
      return;
    }

    setLoading(true);
    setActiveTab('results');

    try {
      const response = await fetch('http://localhost:8081/api/query', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          query: queryText,
          query_type: 'sql',
        }),
      });

      const result = await response.json();
      setResults(result);

      // Add to history
      const historyItem: QueryHistoryItem = {
        id: result.query_id || Date.now().toString(),
        query: queryText,
        timestamp: new Date(),
        duration_ms: result.duration_ms || 0,
        success: result.success,
      };
      setHistory(prev => [historyItem, ...prev].slice(0, 50));

    } catch (error) {
      setResults({
        success: false,
        error: error instanceof Error ? error.message : 'Unknown error',
      });
    } finally {
      setLoading(false);
    }
  };

  const formatQuery = () => {
    if (editorRef.current) {
      editorRef.current.getAction('editor.action.formatDocument').run();
    }
  };

  const renderResults = () => {
    if (!results) {
      return (
        <div className="text-center text-gray-500 py-12">
          <p>Execute a query to see results</p>
        </div>
      );
    }

    if (!results.success) {
      return (
        <div className="bg-red-50 border border-red-200 rounded p-4">
          <h4 className="font-semibold text-red-800 mb-2">Error</h4>
          <pre className="text-red-600 text-sm whitespace-pre-wrap">{results.error}</pre>
        </div>
      );
    }

    if (!results.data) {
      return (
        <div className="bg-green-50 border border-green-200 rounded p-4">
          <p className="text-green-800">
            Query executed successfully. {results.rows_affected || 0} row(s) affected.
          </p>
          <p className="text-sm text-green-600 mt-1">
            Execution time: {results.duration_ms}ms
          </p>
        </div>
      );
    }

    const data = Array.isArray(results.data) ? results.data : [results.data];
    
    if (data.length === 0) {
      return (
        <div className="text-center text-gray-500 py-8">
          <p>No results found</p>
        </div>
      );
    }

    const columns = Object.keys(data[0]);

    return (
      <div>
        <div className="mb-2 flex justify-between items-center">
          <span className="text-sm text-gray-600">
            {data.length} row(s) • {results.duration_ms}ms
          </span>
          <Button variant="secondary" size="sm" onClick={() => {
            const csv = [
              columns.join(','),
              ...data.map(row => columns.map(col => JSON.stringify(row[col])).join(','))
            ].join('\n');
            const blob = new Blob([csv], { type: 'text/csv' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'query_results.csv';
            a.click();
          }}>
            Export CSV
          </Button>
        </div>
        <div className="overflow-x-auto border rounded">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                {columns.map(col => (
                  <th
                    key={col}
                    className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                  >
                    {col}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {data.map((row, idx) => (
                <tr key={idx} className="hover:bg-gray-50">
                  {columns.map(col => (
                    <td key={col} className="px-6 py-3 whitespace-nowrap text-sm text-gray-900">
                      {JSON.stringify(row[col])}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  };

  const renderHistory = () => {
    if (history.length === 0) {
      return (
        <div className="text-center text-gray-500 py-12">
          <p>No query history yet</p>
        </div>
      );
    }

    return (
      <div className="space-y-2">
        {history.map(item => (
          <div
            key={item.id}
            className="border rounded p-3 hover:bg-gray-50 cursor-pointer"
            onClick={() => setQuery(item.query)}
          >
            <div className="flex justify-between items-start mb-2">
              <span className={`text-xs px-2 py-1 rounded ${
                item.success ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
              }`}>
                {item.success ? 'Success' : 'Failed'}
              </span>
              <span className="text-xs text-gray-500">
                {item.timestamp.toLocaleTimeString()} • {item.duration_ms}ms
              </span>
            </div>
            <pre className="text-sm text-gray-700 whitespace-pre-wrap font-mono">
              {item.query}
            </pre>
          </div>
        ))}
      </div>
    );
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <div className="flex justify-between items-center">
            <CardTitle>SQL Editor</CardTitle>
            <div className="flex gap-2">
              <Button variant="secondary" onClick={formatQuery}>
                Format
              </Button>
              <Button 
                onClick={executeQuery} 
                disabled={loading}
              >
                {loading ? 'Executing...' : 'Execute (Ctrl+Enter)'}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="border rounded overflow-hidden">
            <Editor
              height="300px"
              defaultLanguage="sql"
              value={query}
              onChange={(value) => setQuery(value || '')}
              onMount={handleEditorMount}
              theme="vs-light"
              options={{
                minimap: { enabled: false },
                fontSize: 14,
                lineNumbers: 'on',
                scrollBeyondLastLine: false,
                automaticLayout: true,
                tabSize: 2,
              }}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex gap-4 border-b">
            <button
              className={`px-4 py-2 -mb-px ${
                activeTab === 'results'
                  ? 'border-b-2 border-mantis-600 text-mantis-600 font-medium'
                  : 'text-gray-600 hover:text-gray-900'
              }`}
              onClick={() => setActiveTab('results')}
            >
              Results
            </button>
            <button
              className={`px-4 py-2 -mb-px ${
                activeTab === 'history'
                  ? 'border-b-2 border-mantis-600 text-mantis-600 font-medium'
                  : 'text-gray-600 hover:text-gray-900'
              }`}
              onClick={() => setActiveTab('history')}
            >
              History ({history.length})
            </button>
          </div>
        </CardHeader>
        <CardContent>
          {activeTab === 'results' ? renderResults() : renderHistory()}
        </CardContent>
      </Card>
    </div>
  );
};

// Monaco types (would typically be in a separate declaration file)
declare const monaco: any;
