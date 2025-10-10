import { useState, useEffect } from 'react';
import { Table2, Plus, Trash2, RefreshCw, Search, ChevronLeft, ChevronRight, Save, X, Download, Database } from 'lucide-react';
import { apiClient } from '../../api/client';
import { Card, CardHeader, CardTitle, CardContent } from '../ui';
import { CreateTableModal } from './CreateTableModal';
import { ManageColumnsModal } from './ManageColumnsModal';

interface TableInfo {
  name: string;
  type: string;
  row_count: number;
  size_bytes: number;
}

interface Row {
  [key: string]: any;
}

export const TableEditor = () => {
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [selectedTable, setSelectedTable] = useState<string>('');
  const [rows, setRows] = useState<Row[]>([]);
  const [columns, setColumns] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>('');
  const [searchTerm, setSearchTerm] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize] = useState(50);
  const [totalCount, setTotalCount] = useState(0);
  const [editingRow, setEditingRow] = useState<number | null>(null);
  const [editedData, setEditedData] = useState<Row>({});
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showManageColumns, setShowManageColumns] = useState(false);

  useEffect(() => {
    loadTables();
  }, []);

  useEffect(() => {
    if (selectedTable) {
      loadTableData();
    }
  }, [selectedTable, page]);

  const loadTables = async () => {
    try {
      setError('');
      const response = await apiClient.getTables();
      if (response.success && response.data?.tables) {
        setTables(response.data.tables);
        if (response.data.tables.length > 0 && !selectedTable) {
          setSelectedTable(response.data.tables[0].name);
        }
      } else {
        setError(response.error || 'Failed to load tables');
      }
    } catch (err) {
      console.error('Failed to load tables:', err);
      setError('Failed to connect to database');
    }
  };

  const loadTableData = async () => {
    if (!selectedTable) return;
    
    setLoading(true);
    setError('');
    
    try {
      const response = await apiClient.getTableData(selectedTable, {
        limit: pageSize,
        offset: (page - 1) * pageSize,
      });

      if (response.success && response.data) {
        const rowData = response.data.data || [];
        setRows(rowData);
        setTotalCount(response.data.total_count || 0);
        if (rowData.length > 0) {
          setColumns(Object.keys(rowData[0]));
        } else {
          // Load schema columns when table is empty
          const schema = await apiClient.getTableSchema(selectedTable);
          if (schema.success && schema.data?.columns?.length) {
            const cols = schema.data.columns.map((c: any) => c.name);
            setColumns(cols);
          } else {
            setColumns([]);
          }
        }
      } else {
        setError(response.error || 'Failed to load table data');
        setRows([]);
        setColumns([]);
      }
    } catch (err) {
      console.error('Failed to load table data:', err);
      setError('Failed to load table data');
      setRows([]);
      setColumns([]);
    } finally {
      setLoading(false);
    }
  };

  const handleInsertRow = async () => {
    if (!selectedTable || columns.length === 0) return;
    
    const newRow: Record<string, any> = {};
    columns.forEach(col => {
      newRow[col] = col === 'id' ? Date.now() : '';
    });
    
    try {
      const response = await apiClient.createTableData(selectedTable, newRow);
      
      if (response.success) {
        await loadTableData();
      } else {
        setError(response.error || 'Failed to insert row');
      }
    } catch (err) {
      console.error('Failed to insert row:', err);
      setError('Failed to insert row');
    }
  };

  const handleEditRow = (index: number) => {
    setEditingRow(index);
    setEditedData({ ...rows[index] });
  };

  const handleSaveRow = async (index: number) => {
    if (!selectedTable) return;
    
    const row = rows[index];
    const rowId = row.id || index;
    
    try {
      const response = await apiClient.updateTableData(selectedTable, String(rowId), editedData);
      
      if (response.success) {
        setEditingRow(null);
        setEditedData({});
        await loadTableData();
      } else {
        setError(response.error || 'Failed to update row');
      }
    } catch (err) {
      console.error('Failed to update row:', err);
      setError('Failed to update row');
    }
  };

  const handleCancelEdit = () => {
    setEditingRow(null);
    setEditedData({});
  };

  const handleDeleteRow = async (index: number) => {
    if (!selectedTable || !confirm('Are you sure you want to delete this row?')) return;
    
    const row = rows[index];
    const rowId = row.id || index;
    
    try {
      const response = await apiClient.deleteTableData(selectedTable, String(rowId));
      
      if (response.success) {
        await loadTableData();
      } else {
        setError(response.error || 'Failed to delete row');
      }
    } catch (err) {
      console.error('Failed to delete row:', err);
      setError('Failed to delete row');
    }
  };

  const handleCellEdit = (column: string, value: any) => {
    setEditedData(prev => ({ ...prev, [column]: value }));
  };

  const handleCreateTable = async (name: string, type: string, columns: any[]) => {
    try {
      const response = await apiClient.createTable(name, type, columns);
      if (response.success) {
        await loadTables();
        setSelectedTable(name);
      } else {
        throw new Error(response.error || 'Failed to create table');
      }
    } catch (err) {
      throw err;
    }
  };

  const handleExportCSV = () => {
    if (filteredRows.length === 0) return;
    
    const headers = columns.join(',');
    const csvRows = filteredRows.map(row => 
      columns.map(col => {
        const value = row[col];
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

  const handleExportJSON = () => {
    if (filteredRows.length === 0) return;
    
    const json = JSON.stringify(filteredRows, null, 2);
    const blob = new Blob([json], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${selectedTable}_${new Date().toISOString().split('T')[0]}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const filteredRows = rows.filter(row =>
    searchTerm === '' || Object.values(row).some(val =>
      String(val).toLowerCase().includes(searchTerm.toLowerCase())
    )
  );

  const formatValue = (value: any): string => {
    if (value === null || value === undefined) return 'NULL';
    if (typeof value === 'object') return JSON.stringify(value);
    if (typeof value === 'boolean') return value ? 'true' : 'false';
    return String(value);
  };

  const totalPages = Math.ceil(totalCount / pageSize);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">Table Editor</h2>
          <p className="text-sm text-gray-600 mt-1">
            View and edit table data across all storage models
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowCreateModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-mantis-600 hover:bg-mantis-700 text-white rounded-lg transition-colors"
          >
            <Database className="w-4 h-4" />
            Create Table
          </button>
          <button
            onClick={loadTables}
            className="flex items-center gap-2 px-4 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-lg transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
        </div>
      </div>

      {/* Table Selector */}
      <Card>
        <CardHeader>
          <CardTitle>Select Table</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            {tables.map((table) => (
              <button
                key={table.name}
                onClick={() => {
                  setSelectedTable(table.name);
                  setPage(1);
                }}
                className={`p-4 rounded-lg border-2 transition-all text-left ${
                  selectedTable === table.name
                    ? 'border-mantis-600 bg-mantis-50'
                    : 'border-gray-200 hover:border-gray-300'
                }`}
              >
                <div className="flex items-center gap-2 mb-2">
                  <Table2 className="w-4 h-4" />
                  <span className="font-medium">{table.name}</span>
                </div>
                <div className="text-xs text-gray-600">
                  {table.row_count.toLocaleString()} rows
                </div>
              </button>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Table Data */}
      {selectedTable && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>{selectedTable}</CardTitle>
              <div className="flex items-center gap-2">
                <div className="flex items-center gap-1 mr-2">
                  <button
                    onClick={handleExportCSV}
                    disabled={filteredRows.length === 0}
                    className="flex items-center gap-2 px-3 py-2 text-sm bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    title="Export as CSV"
                  >
                    <Download className="w-4 h-4" />
                    CSV
                  </button>
                  <button
                    onClick={handleExportJSON}
                    disabled={filteredRows.length === 0}
                    className="flex items-center gap-2 px-3 py-2 text-sm bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    title="Export as JSON"
                  >
                    <Download className="w-4 h-4" />
                    JSON
                  </button>
                </div>
                <button
                  onClick={() => setShowManageColumns(true)}
                  className="flex items-center gap-2 px-4 py-2 border rounded-lg hover:bg-gray-50 transition-colors"
                >
                  Manage Columns
                </button>
                <button
                  onClick={handleInsertRow}
                  className="flex items-center gap-2 px-4 py-2 bg-mantis-600 hover:bg-mantis-700 text-white rounded-lg transition-colors"
                >
                  <Plus className="w-4 h-4" />
                  Insert Row
                </button>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            {/* Search and Pagination */}
            <div className="flex items-center justify-between mb-4">
              <div className="flex-1 max-w-md relative">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input
                  type="text"
                  placeholder="Search in table..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-600"
                />
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setPage(Math.max(1, page - 1))}
                  disabled={page === 1}
                  className="p-2 border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>
                <span className="text-sm text-gray-600 px-3">
                  Page {page} of {totalPages || 1}
                </span>
                <button
                  onClick={() => setPage(page + 1)}
                  disabled={page >= totalPages}
                  className="p-2 border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <ChevronRight className="w-4 h-4" />
                </button>
              </div>
            </div>

            {/* Error Message */}
            {error && (
              <div className="mb-4 p-4 bg-red-50 border border-red-200 rounded-lg text-red-700">
                {error}
              </div>
            )}

            {/* Table */}
            {loading ? (
              <div className="flex items-center justify-center py-12">
                <div className="text-gray-400">Loading...</div>
              </div>
            ) : filteredRows.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12">
                <Table2 className="w-16 h-16 text-gray-300 mb-4" />
                <p className="text-gray-500 text-lg mb-2">No rows found</p>
                <button
                  onClick={handleInsertRow}
                  className="flex items-center gap-2 px-4 py-2 bg-mantis-600 hover:bg-mantis-700 text-white rounded-lg transition-colors"
                >
                  <Plus className="w-4 h-4" />
                  Insert First Row
                </button>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead className="bg-gray-50 border-b border-gray-200">
                    <tr>
                      {columns.map((column) => (
                        <th
                          key={column}
                          className="px-6 py-3 text-left text-xs font-medium text-gray-600 uppercase tracking-wider"
                        >
                          {column}
                        </th>
                      ))}
                      <th className="px-6 py-3 text-right text-xs font-medium text-gray-600 uppercase tracking-wider">
                        Actions
                      </th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {filteredRows.map((row, index) => (
                      <tr key={index} className="hover:bg-gray-50">
                        {columns.map((column) => (
                          <td key={column} className="px-6 py-4 text-sm text-gray-900">
                            {editingRow === index ? (
                              <input
                                type="text"
                                value={editedData[column] ?? row[column] ?? ''}
                                onChange={(e) => handleCellEdit(column, e.target.value)}
                                className="w-full px-2 py-1 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-mantis-600"
                              />
                            ) : (
                              <div className="max-w-xs truncate" title={formatValue(row[column])}>
                                {row[column] === null || row[column] === undefined ? (
                                  <span className="text-gray-400 italic">NULL</span>
                                ) : typeof row[column] === 'object' ? (
                                  <span className="text-mantis-600 font-mono text-xs">
                                    {JSON.stringify(row[column])}
                                  </span>
                                ) : (
                                  formatValue(row[column])
                                )}
                              </div>
                            )}
                          </td>
                        ))}
                        <td className="px-6 py-4 text-right text-sm">
                          {editingRow === index ? (
                            <div className="flex items-center justify-end gap-2">
                              <button
                                onClick={() => handleSaveRow(index)}
                                className="p-1.5 text-green-600 hover:bg-green-50 rounded transition-colors"
                                title="Save"
                              >
                                <Save className="w-4 h-4" />
                              </button>
                              <button
                                onClick={handleCancelEdit}
                                className="p-1.5 text-gray-600 hover:bg-gray-100 rounded transition-colors"
                                title="Cancel"
                              >
                                <X className="w-4 h-4" />
                              </button>
                            </div>
                          ) : (
                            <div className="flex items-center justify-end gap-2">
                              <button
                                onClick={() => handleEditRow(index)}
                                className="px-3 py-1 text-sm text-mantis-600 hover:bg-mantis-50 rounded transition-colors"
                              >
                                Edit
                              </button>
                              <button
                                onClick={() => handleDeleteRow(index)}
                                className="p-1.5 text-red-600 hover:bg-red-50 rounded transition-colors"
                                title="Delete"
                              >
                                <Trash2 className="w-4 h-4" />
                              </button>
                            </div>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {/* Footer */}
            {filteredRows.length > 0 && (
              <div className="mt-4 flex items-center justify-between text-sm text-gray-600">
                <div>
                  Showing {((page - 1) * pageSize) + 1} to {Math.min(page * pageSize, totalCount)} of {totalCount} rows
                </div>
                <div>
                  {filteredRows.length} {filteredRows.length === 1 ? 'row' : 'rows'} displayed
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Create Table Modal */}
      <CreateTableModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSubmit={handleCreateTable}
      />
      <ManageColumnsModal
        isOpen={showManageColumns}
        tableName={selectedTable}
        onClose={() => setShowManageColumns(false)}
        onSaved={async () => { await loadTableData(); }}
      />
    </div>
  );
};
