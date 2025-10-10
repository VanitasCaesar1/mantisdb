import React, { useState, useRef, useEffect } from 'react';
import Editor from '@monaco-editor/react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';
import { apiClient } from '../../api/client';

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

interface SchemaInfo {
  tables: Array<{
    name: string;
    columns: Array<{
      name: string;
      type: string;
    }>;
  }>;
}

export const EnhancedSQLEditor: React.FC = () => {
  const [query, setQuery] = useState('SELECT * FROM users LIMIT 10;');
  const [results, setResults] = useState<QueryResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [history, setHistory] = useState<QueryHistoryItem[]>([]);
  const [activeTab, setActiveTab] = useState<'results' | 'history' | 'explain'>('results');
  const [schema, setSchema] = useState<SchemaInfo | null>(null);
  const [showSavedQueries, setShowSavedQueries] = useState(false);
  const [savedQueries, setSavedQueries] = useState<Array<{name: string; query: string}>>([]);
  const editorRef = useRef<any>(null);
  const monacoRef = useRef<any>(null);

  // Load schema for autocomplete
  useEffect(() => {
    loadSchema();
  }, []);

  const loadSchema = async () => {
    try {
      const response = await apiClient.getTables();
      
      if (response.success && response.data?.tables) {
        // Transform tables to match expected schema format
        const tables = response.data.tables.map(table => ({
          name: table.name,
          columns: (table.columns || []).map(col => ({
            name: col.name,
            type: col.type
          }))
        }));
        setSchema({ tables });
      }
    } catch (error) {
      console.error('Failed to load schema:', error);
    }
  };

  const handleEditorMount = (editor: any, monaco: any) => {
    editorRef.current = editor;
    monacoRef.current = monaco;
    
    // Register SQL language features
    monaco.languages.registerCompletionItemProvider('sql', {
      provideCompletionItems: (_model: any, _position: any) => {
        const suggestions: any[] = [];
        
        // SQL Keywords
        const keywords = [
          'SELECT', 'FROM', 'WHERE', 'INSERT', 'UPDATE', 'DELETE', 'CREATE', 'DROP',
          'TABLE', 'INDEX', 'JOIN', 'LEFT', 'RIGHT', 'INNER', 'OUTER', 'ON',
          'GROUP BY', 'ORDER BY', 'HAVING', 'LIMIT', 'OFFSET', 'AS', 'AND', 'OR',
          'NOT', 'IN', 'LIKE', 'BETWEEN', 'IS', 'NULL', 'DISTINCT', 'COUNT',
          'SUM', 'AVG', 'MIN', 'MAX', 'ASC', 'DESC'
        ];
        
        keywords.forEach(keyword => {
          suggestions.push({
            label: keyword,
            kind: monaco.languages.CompletionItemKind.Keyword,
            insertText: keyword,
            detail: 'SQL Keyword'
          });
        });
        
        // Table names
        if (schema?.tables) {
          schema.tables.forEach(table => {
            suggestions.push({
              label: table.name,
              kind: monaco.languages.CompletionItemKind.Class,
              insertText: table.name,
              detail: 'Table',
              documentation: `Columns: ${table.columns.map(c => c.name).join(', ')}`
            });
            
            // Column names
            table.columns.forEach(column => {
              suggestions.push({
                label: `${table.name}.${column.name}`,
                kind: monaco.languages.CompletionItemKind.Field,
                insertText: column.name,
                detail: `Column (${column.type})`,
                documentation: `Table: ${table.name}, Type: ${column.type}`
              });
            });
          });
        }
        
        // SQL Functions
        const functions = [
          'COUNT', 'SUM', 'AVG', 'MIN', 'MAX', 'UPPER', 'LOWER', 'LENGTH',
          'SUBSTRING', 'CONCAT', 'COALESCE', 'CAST', 'NOW', 'DATE', 'YEAR'
        ];
        
        functions.forEach(func => {
          suggestions.push({
            label: func,
            kind: monaco.languages.CompletionItemKind.Function,
            insertText: `${func}()`,
            detail: 'SQL Function'
          });
        });
        
        return { suggestions };
      }
    });
    
    // Add custom keybindings
    editor.addAction({
      id: 'execute-query',
      label: 'Execute Query',
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter],
      run: () => executeQuery(),
    });
    
    editor.addAction({
      id: 'format-query',
      label: 'Format Query',
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyF],
      run: () => formatQuery(),
    });
    
    editor.addAction({
      id: 'save-query',
      label: 'Save Query',
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS],
      run: () => saveQuery(),
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
      const response = await apiClient.executeQuery({
        query: queryText,
        query_type: 'sql',
      });

      const result: any = response.data || response;
      setResults(result);

      // Add to history
      const historyItem: QueryHistoryItem = {
        id: (result?.query_id || Date.now().toString()) as string,
        query: queryText,
        timestamp: new Date(),
        duration_ms: result?.duration_ms || 0,
        success: result?.success || false,
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

  const saveQuery = () => {
    const queryText = editorRef.current?.getValue() || query;
    const name = prompt('Enter a name for this query:');
    
    if (name) {
      setSavedQueries(prev => [...prev, { name, query: queryText }]);
      alert('Query saved successfully!');
    }
  };

  const loadSavedQuery = (savedQuery: {name: string; query: string}) => {
    setQuery(savedQuery.query);
    if (editorRef.current) {
      editorRef.current.setValue(savedQuery.query);
    }
    setShowSavedQueries(false);
  };

  const explainQuery = async () => {
    setActiveTab('explain');
    
    // Mock explain plan - in production, get from backend
    setResults({
      success: true,
      data: {
        plan: [
          { step: 1, operation: 'Seq Scan', table: 'users', cost: '0.00..10.50', rows: 100 },
          { step: 2, operation: 'Filter', condition: 'id > 10', rows: 50 },
          { step: 3, operation: 'Sort', key: 'created_at DESC', rows: 50 }
        ]
      }
    });
  };

  const renderResults = () => {
    if (!results) {
      return (
        <div className="text-center text-gray-500 py-12">
          <p>Execute a query to see results</p>
          <div className="mt-4 text-sm">
            <p className="font-semibold mb-2">Quick Tips:</p>
            <ul className="text-left inline-block">
              <li>• Press <kbd className="px-2 py-1 bg-gray-100 rounded">Ctrl+Enter</kbd> to execute</li>
              <li>• Press <kbd className="px-2 py-1 bg-gray-100 rounded">Ctrl+Space</kbd> for autocomplete</li>
              <li>• Press <kbd className="px-2 py-1 bg-gray-100 rounded">Ctrl+S</kbd> to save query</li>
            </ul>
          </div>
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
          <div className="flex gap-2">
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
            <Button variant="secondary" size="sm" onClick={() => {
              const json = JSON.stringify(data, null, 2);
              const blob = new Blob([json], { type: 'application/json' });
              const url = URL.createObjectURL(blob);
              const a = document.createElement('a');
              a.href = url;
              a.download = 'query_results.json';
              a.click();
            }}>
              Export JSON
            </Button>
          </div>
        </div>
        <div className="overflow-x-auto border rounded max-h-96 overflow-y-auto">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50 sticky top-0">
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

  const renderExplainPlan = () => {
    if (!results?.data?.plan) {
      return <div className="text-center text-gray-500 py-12">No explain plan available</div>;
    }

    return (
      <div className="space-y-2">
        <h4 className="font-semibold mb-4">Query Execution Plan</h4>
        {results.data.plan.map((step: any, idx: number) => (
          <div key={idx} className="border rounded p-4 hover:bg-gray-50">
            <div className="flex items-start gap-4">
              <div className="flex-shrink-0 w-8 h-8 bg-mantis-100 rounded-full flex items-center justify-center text-mantis-700 font-bold">
                {step.step}
              </div>
              <div className="flex-1">
                <div className="font-semibold text-gray-900">{step.operation}</div>
                {step.table && <div className="text-sm text-gray-600">Table: {step.table}</div>}
                {step.condition && <div className="text-sm text-gray-600">Condition: {step.condition}</div>}
                {step.key && <div className="text-sm text-gray-600">Sort Key: {step.key}</div>}
                <div className="text-xs text-gray-500 mt-1">
                  Cost: {step.cost} • Rows: {step.rows}
                </div>
              </div>
            </div>
          </div>
        ))}
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
            onClick={() => {
              setQuery(item.query);
              if (editorRef.current) {
                editorRef.current.setValue(item.query);
              }
            }}
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
            <CardTitle>Enhanced SQL Editor</CardTitle>
            <div className="flex gap-2">
              <Button variant="secondary" onClick={() => setShowSavedQueries(!showSavedQueries)}>
                Saved Queries ({savedQueries.length})
              </Button>
              <Button variant="secondary" onClick={explainQuery}>
                Explain
              </Button>
              <Button variant="secondary" onClick={formatQuery}>
                Format
              </Button>
              <Button variant="secondary" onClick={saveQuery}>
                Save
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
          {showSavedQueries && (
            <div className="mb-4 p-4 bg-gray-50 rounded">
              <h4 className="font-semibold mb-2">Saved Queries</h4>
              {savedQueries.length === 0 ? (
                <p className="text-sm text-gray-500">No saved queries yet</p>
              ) : (
                <div className="space-y-2">
                  {savedQueries.map((sq, idx) => (
                    <div key={idx} className="flex justify-between items-center p-2 bg-white rounded hover:bg-gray-100 cursor-pointer" onClick={() => loadSavedQuery(sq)}>
                      <span className="font-medium">{sq.name}</span>
                      <Button variant="secondary" size="sm">Load</Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
          <div className="border rounded overflow-hidden">
            <Editor
              height="400px"
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
                suggestOnTriggerCharacters: true,
                quickSuggestions: true,
                wordBasedSuggestions: false,
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
                activeTab === 'explain'
                  ? 'border-b-2 border-mantis-600 text-mantis-600 font-medium'
                  : 'text-gray-600 hover:text-gray-900'
              }`}
              onClick={() => setActiveTab('explain')}
            >
              Explain Plan
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
          {activeTab === 'results' && renderResults()}
          {activeTab === 'explain' && renderExplainPlan()}
          {activeTab === 'history' && renderHistory()}
        </CardContent>
      </Card>
    </div>
  );
};
