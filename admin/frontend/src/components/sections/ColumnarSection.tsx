import { useState, useEffect } from 'react';
import { Columns, Plus, Trash2, RefreshCw, Download, TrendingUp } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '../ui';
import { apiClient } from '../../api/client';

interface ColumnarTable {
  name: string;
  columns: ColumnInfo[];
  row_count: number;
  size_bytes: number;
  partitions?: number;
}

interface ColumnInfo {
  name: string;
  type: string;
  primary_key?: boolean;
  clustering_key?: boolean;
}


export const ColumnarSection = () => {
  const [tables, setTables] = useState<ColumnarTable[]>([]);
  const [selectedTable, setSelectedTable] = useState<string>('');
  const [rows, setRows] = useState<any[]>([]);
  const [columns, setColumns] = useState<ColumnInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>('');
  const [showQueryModal, setShowQueryModal] = useState(false);
  const [cqlQuery, setCqlQuery] = useState('');
  const filters: Record<string, string> = {};
  const limit = 100;
  const offset = 0;

  useEffect(() => {
    loadTables();
  }, []);

  useEffect(() => {
    if (selectedTable) {
      loadTableData();
    }
  }, [selectedTable, offset]);

  const loadTables = async () => {
    try {
      setError('');
      const response = await apiClient.getColumnarTables();
      if (response.success && response.data?.tables) {
        setTables(response.data.tables.map(t => ({
          ...t,
          columns: t.columns || []
        })));
        if (response.data.tables.length > 0 && !selectedTable) {
          setSelectedTable(response.data.tables[0].name);
        }
      }
    } catch (err) {
      console.error('Failed to load tables:', err);
      setError('Failed to load columnar tables');
    }
  };

  const loadTableData = async () => {
    if (!selectedTable) return;
    
    setLoading(true);
    setError('');
    
    try {
      const response = await apiClient.queryColumnarTable(selectedTable, {
        filters,
        limit,
        offset,
      });

      if (response.success && response.data) {
        setRows(response.data.rows || []);
        if (response.data.rows && response.data.rows.length > 0) {
          const firstRow = response.data.rows[0];
          const cols = Object.keys(firstRow).map(name => ({
            name,
            type: typeof firstRow[name],
          }));
          setColumns(cols);
        }
      } else {
        setError(response.error || 'Failed to load table data');
      }
    } catch (err) {
      console.error('Failed to load table data:', err);
      setError('Failed to load table data');
    } finally {
      setLoading(false);
    }
  };

  const handleExecuteCQL = async () => {
    if (!cqlQuery.trim()) return;
    setLoading(true);
    setError('');
    try {
      const response = await apiClient.executeCql(cqlQuery);
      if (response.success) {
        const rows = response.data?.rows || [];
        setRows(rows);
        // Update columns based on result
        if (rows.length > 0) {
          const first = rows[0];
          const cols = Object.keys(first).map(name => ({ name, type: typeof first[name] }));
          setColumns(cols);
        }
        setShowQueryModal(false);
        setCqlQuery('');
      } else {
        setError(response.error || 'Failed to execute CQL query');
      }
    } catch (err) {
      setError('Failed to execute CQL query');
    } finally {
      setLoading(false);
    }
  };

  const handleCreateTable = async () => {
    const name = prompt('Enter table name');
    if (!name) return;
    const colsInput = prompt('Enter columns (comma-separated, e.g., id:int,name:text)') || '';
    const columns = colsInput.split(',').map(s => s.trim()).filter(Boolean).map(def => {
      const [n, t] = def.split(':');
      return {
        name: n || 'col',
        data_type: (t || 'text'),
        nullable: false,
        indexed: false,
        primary_key: n?.toLowerCase() === 'id',
      };
    });
    try {
      const res = await apiClient.createColumnarTable(name, columns);
      if (res.success) {
        await loadTables();
        setSelectedTable(name);
      } else {
        setError(res.error || 'Failed to create table');
      }
    } catch (e) {
      setError('Failed to create table');
    }
  };

  const handleInsertRow = async () => {
    if (!selectedTable || columns.length === 0) return;
    
    const newRow: Record<string, any> = {};
    columns.forEach(col => {
      newRow[col.name] = col.name === 'id' ? Date.now() : '';
    });
    
    try {
      const response = await apiClient.insertColumnarRows(selectedTable, [newRow]);
      if (response.success) {
        await loadTableData();
      } else {
        setError(response.error || 'Failed to insert row');
      }
    } catch (err) {
      setError('Failed to insert row');
    }
  };


  const handleExportCSV = () => {
    if (rows.length === 0) return;
    
    const headers = columns.map(c => c.name).join(',');
    const csvRows = rows.map(row => 
      columns.map(col => {
        const value = row[col.name];
        if (value === null || value === undefined) return '';
        if (typeof value === 'object') return JSON.stringify(value).replace(/"/g, '""');
        return String(value).replace(/"/g, '""');
      }).join(',')
    );
    
    const csv = [headers, ...csvRows].join('\n');
    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${selectedTable}_${new Date().toISOString().split('T')[0]}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">Columnar Store</h2>
          <p className="text-sm text-gray-600 mt-1">Cassandra/ScyllaDB-style wide-column storage</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowQueryModal(true)}
            className="flex items-center gap-2 px-4 py-2 border rounded-lg hover:bg-gray-50 transition-colors"
          >
            <Columns className="w-4 h-4" />
            CQL Query
          </button>
          <button
            onClick={handleCreateTable}
            className="flex items-center gap-2 px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors"
          >
            <Plus className="w-4 h-4" />
            New Table
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
          {error}
        </div>
      )}

      <div className="grid grid-cols-12 gap-6">
        {/* Tables Sidebar */}
        <div className="col-span-3">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Tables</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-1">
                {tables.map((table) => (
                  <button
                    key={table.name}
                    onClick={() => setSelectedTable(table.name)}
                    className={`w-full text-left px-3 py-2 rounded-lg transition-colors ${
                      selectedTable === table.name
                        ? 'bg-mantis-100 text-mantis-900 font-medium'
                        : 'hover:bg-gray-100 text-gray-700'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <Columns className="w-4 h-4" />
                      <div className="flex-1 min-w-0">
                        <div className="text-sm truncate">{table.name}</div>
                        <div className="text-xs text-gray-500">
                          {table.row_count} rows · {table.columns?.length || 0} cols
                        </div>
                      </div>
                    </div>
                  </button>
                ))}
                {tables.length === 0 && (
                  <div className="text-sm text-gray-500 text-center py-4">
                    No tables yet
                  </div>
                )}
              </div>
            </CardContent>
          </Card>

          {/* Table Info */}
          {selectedTable && (
            <Card className="mt-4">
              <CardHeader>
                <CardTitle className="text-sm">Schema</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {columns.map((col) => (
                    <div key={col.name} className="flex items-center justify-between text-xs">
                      <span className="font-mono text-gray-700">{col.name}</span>
                      <span className="text-gray-500">{col.type}</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}
        </div>

        {/* Data View */}
        <div className="col-span-9">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>
                  {selectedTable || 'Select a table'}
                </CardTitle>
                <div className="flex items-center gap-2">
                  <button
                    onClick={loadTableData}
                    disabled={loading}
                    className="p-2 border rounded-lg hover:bg-gray-50 transition-colors"
                  >
                    <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
                  </button>
                  <button
                    onClick={handleExportCSV}
                    disabled={rows.length === 0}
                    className="p-2 border rounded-lg hover:bg-gray-50 transition-colors disabled:opacity-50"
                  >
                    <Download className="w-4 h-4" />
                  </button>
                  <button
                    onClick={handleInsertRow}
                    disabled={!selectedTable}
                    className="flex items-center gap-2 px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors disabled:opacity-50"
                  >
                    <Plus className="w-4 h-4" />
                    Insert Row
                  </button>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="text-center py-12 text-gray-500">
                  <RefreshCw className="w-8 h-8 animate-spin mx-auto mb-2" />
                  Loading data...
                </div>
              ) : rows.length === 0 ? (
                <div className="text-center py-12 text-gray-500">
                  <TrendingUp className="w-12 h-12 mx-auto mb-2 opacity-50" />
                  <p>No data found</p>
                  {selectedTable && (
                    <button
                      onClick={handleInsertRow}
                      className="mt-4 text-mantis-600 hover:text-mantis-700"
                    >
                      Insert your first row
                    </button>
                  )}
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b">
                        {columns.map((col) => (
                          <th key={col.name} className="text-left py-3 px-4 font-medium text-gray-700">
                            {col.name}
                            {col.primary_key && <span className="ml-1 text-xs text-mantis-600">PK</span>}
                          </th>
                        ))}
                        <th className="text-right py-3 px-4 font-medium text-gray-700">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((row, idx) => (
                        <tr key={idx} className="border-b hover:bg-gray-50">
                          {columns.map((col) => (
                            <td key={col.name} className="py-3 px-4 font-mono text-xs">
                              {row[col.name] !== null && row[col.name] !== undefined
                                ? typeof row[col.name] === 'object'
                                  ? JSON.stringify(row[col.name])
                                  : String(row[col.name])
                                : <span className="text-gray-400">NULL</span>
                              }
                            </td>
                          ))}
                          <td className="py-3 px-4 text-right">
                            <button className="p-1 hover:bg-red-50 rounded">
                              <Trash2 className="w-4 h-4 text-red-600" />
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* CQL Query Modal */}
      {showQueryModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 max-w-2xl w-full mx-4">
            <h3 className="text-lg font-semibold mb-4">Execute CQL Query</h3>
            <textarea
              value={cqlQuery}
              onChange={(e) => setCqlQuery(e.target.value)}
              className="w-full h-48 font-mono text-sm border rounded-lg p-3"
              placeholder="SELECT * FROM table_name WHERE column = 'value';"
            />
            <div className="flex justify-end gap-3 mt-4">
              <button
                onClick={() => setShowQueryModal(false)}
                className="px-4 py-2 border rounded-lg hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={handleExecuteCQL}
                className="px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700"
              >
                Execute
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
