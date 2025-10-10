import React, { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';

interface Table {
  name: string;
  schema: string;
  row_count: number;
  columns: Column[];
}

interface Column {
  name: string;
  type: string;
  nullable: boolean;
  default_value?: string;
  is_primary_key: boolean;
}

interface Row {
  [key: string]: any;
}

export const SupabaseStyleBrowser: React.FC = () => {
  const [tables, setTables] = useState<Table[]>([]);
  const [selectedTable, setSelectedTable] = useState<Table | null>(null);
  const [rows, setRows] = useState<Row[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(100);
  const [totalRows, setTotalRows] = useState(0);
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [sortColumn, setSortColumn] = useState<string | null>(null);
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('asc');
  const [selectedRows, setSelectedRows] = useState<Set<number>>(new Set());
  const [showInsertModal, setShowInsertModal] = useState(false);
  const [editingRow, setEditingRow] = useState<Row | null>(null);

  useEffect(() => {
    loadTables();
  }, []);

  useEffect(() => {
    if (selectedTable) {
      loadRows();
    }
  }, [selectedTable, page, filters, sortColumn, sortDirection]);

  const loadTables = async () => {
    try {
      const response = await fetch('http://localhost:8081/api/tables');
      const data = await response.json();
      if (data.success && data.tables) {
        setTables(data.tables);
        if (data.tables.length > 0 && !selectedTable) {
          setSelectedTable(data.tables[0]);
        }
      }
    } catch (error) {
      console.error('Failed to load tables:', error);
    }
  };

  const loadRows = async () => {
    if (!selectedTable) return;
    
    setLoading(true);
    try {
      const params = new URLSearchParams({
        limit: pageSize.toString(),
        offset: ((page - 1) * pageSize).toString(),
      });

      if (sortColumn) {
        params.append('sort', `${sortColumn}:${sortDirection}`);
      }

      Object.entries(filters).forEach(([key, value]) => {
        if (value) {
          params.append(`filter[${key}]`, value);
        }
      });

      const response = await fetch(
        `http://localhost:8081/api/tables/${selectedTable.name}/data?${params}`
      );
      const data = await response.json();
      
      if (data.success) {
        setRows(data.rows || []);
        setTotalRows(data.total || 0);
      }
    } catch (error) {
      console.error('Failed to load rows:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSort = (column: string) => {
    if (sortColumn === column) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortColumn(column);
      setSortDirection('asc');
    }
  };

  const handleFilter = (column: string, value: string) => {
    setFilters(prev => ({ ...prev, [column]: value }));
    setPage(1);
  };

  const handleRowSelect = (index: number) => {
    const newSelected = new Set(selectedRows);
    if (newSelected.has(index)) {
      newSelected.delete(index);
    } else {
      newSelected.add(index);
    }
    setSelectedRows(newSelected);
  };

  const handleSelectAll = () => {
    if (selectedRows.size === rows.length) {
      setSelectedRows(new Set());
    } else {
      setSelectedRows(new Set(rows.map((_, i) => i)));
    }
  };

  const handleDeleteSelected = async () => {
    if (!confirm(`Delete ${selectedRows.size} row(s)?`)) return;

    // TODO: Implement bulk delete
    alert('Delete functionality coming soon');
  };

  const handleEditRow = (row: Row) => {
    setEditingRow(row);
    setShowInsertModal(true);
  };

  const handleInsertRow = () => {
    setEditingRow(null);
    setShowInsertModal(true);
  };

  const totalPages = Math.ceil(totalRows / pageSize);

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar - Table List */}
      <div className="w-64 bg-white border-r border-gray-200 overflow-y-auto">
        <div className="p-4 border-b border-gray-200">
          <h2 className="text-lg font-semibold text-gray-900">Tables</h2>
          <p className="text-sm text-gray-500">{tables.length} tables</p>
        </div>
        <div className="p-2">
          {tables.map(table => (
            <button
              key={table.name}
              onClick={() => setSelectedTable(table)}
              className={`w-full text-left px-3 py-2 rounded mb-1 transition-colors ${
                selectedTable?.name === table.name
                  ? 'bg-mantis-100 text-mantis-900 font-medium'
                  : 'hover:bg-gray-100 text-gray-700'
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="truncate">{table.name}</span>
                <span className="text-xs text-gray-500">{table.row_count}</span>
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Header */}
        <div className="bg-white border-b border-gray-200 px-6 py-4">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold text-gray-900">
                {selectedTable?.name || 'Select a table'}
              </h1>
              <p className="text-sm text-gray-500 mt-1">
                {totalRows.toLocaleString()} rows
              </p>
            </div>
            <div className="flex gap-2">
              {selectedRows.size > 0 && (
                <Button variant="secondary" onClick={handleDeleteSelected}>
                  Delete ({selectedRows.size})
                </Button>
              )}
              <Button onClick={handleInsertRow}>
                Insert Row
              </Button>
            </div>
          </div>
        </div>

        {/* Toolbar */}
        <div className="bg-white border-b border-gray-200 px-6 py-3">
          <div className="flex items-center gap-4">
            <div className="flex-1">
              <input
                type="text"
                placeholder="Filter rows..."
                className="w-full px-3 py-2 border border-gray-300 rounded text-sm"
                onChange={(e) => {
                  // Simple global filter
                  const value = e.target.value;
                  if (selectedTable?.columns[0]) {
                    handleFilter(selectedTable.columns[0].name, value);
                  }
                }}
              />
            </div>
            <div className="flex items-center gap-2 text-sm text-gray-600">
              <span>Page {page} of {totalPages}</span>
              <div className="flex gap-1">
                <button
                  onClick={() => setPage(Math.max(1, page - 1))}
                  disabled={page === 1}
                  className="px-2 py-1 border rounded disabled:opacity-50"
                >
                  ←
                </button>
                <button
                  onClick={() => setPage(Math.min(totalPages, page + 1))}
                  disabled={page === totalPages}
                  className="px-2 py-1 border rounded disabled:opacity-50"
                >
                  →
                </button>
              </div>
            </div>
          </div>
        </div>

        {/* Table Content */}
        <div className="flex-1 overflow-auto bg-white">
          {loading ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-gray-500">Loading...</div>
            </div>
          ) : !selectedTable ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-center">
                <p className="text-gray-500 mb-2">Select a table to view data</p>
              </div>
            </div>
          ) : rows.length === 0 ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-center">
                <p className="text-gray-500 mb-2">No rows found</p>
                <Button onClick={handleInsertRow}>Insert First Row</Button>
              </div>
            </div>
          ) : (
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50 sticky top-0">
                <tr>
                  <th className="w-12 px-4 py-3">
                    <input
                      type="checkbox"
                      checked={selectedRows.size === rows.length && rows.length > 0}
                      onChange={handleSelectAll}
                      className="rounded"
                    />
                  </th>
                  {selectedTable.columns.map(column => (
                    <th
                      key={column.name}
                      className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100"
                      onClick={() => handleSort(column.name)}
                    >
                      <div className="flex items-center gap-2">
                        <span>{column.name}</span>
                        {column.is_primary_key && (
                          <span className="text-yellow-600" title="Primary Key">🔑</span>
                        )}
                        {sortColumn === column.name && (
                          <span>{sortDirection === 'asc' ? '↑' : '↓'}</span>
                        )}
                      </div>
                      <div className="text-xs text-gray-400 font-normal mt-1">
                        {column.type}
                      </div>
                    </th>
                  ))}
                  <th className="w-24 px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {rows.map((row, index) => (
                  <tr
                    key={index}
                    className={`hover:bg-gray-50 ${
                      selectedRows.has(index) ? 'bg-blue-50' : ''
                    }`}
                  >
                    <td className="px-4 py-3">
                      <input
                        type="checkbox"
                        checked={selectedRows.has(index)}
                        onChange={() => handleRowSelect(index)}
                        className="rounded"
                      />
                    </td>
                    {selectedTable.columns.map(column => (
                      <td key={column.name} className="px-6 py-3 text-sm text-gray-900">
                        <div className="max-w-xs truncate" title={String(row[column.name])}>
                          {row[column.name] === null ? (
                            <span className="text-gray-400 italic">NULL</span>
                          ) : typeof row[column.name] === 'object' ? (
                            <span className="text-blue-600">
                              {JSON.stringify(row[column.name])}
                            </span>
                          ) : (
                            String(row[column.name])
                          )}
                        </div>
                      </td>
                    ))}
                    <td className="px-4 py-3 text-right text-sm">
                      <button
                        onClick={() => handleEditRow(row)}
                        className="text-blue-600 hover:text-blue-800 mr-3"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => {
                          if (confirm('Delete this row?')) {
                            // TODO: Implement delete
                            alert('Delete functionality coming soon');
                          }
                        }}
                        className="text-red-600 hover:text-red-800"
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Footer */}
        <div className="bg-white border-t border-gray-200 px-6 py-3">
          <div className="flex items-center justify-between text-sm text-gray-600">
            <div>
              Showing {((page - 1) * pageSize) + 1} to {Math.min(page * pageSize, totalRows)} of {totalRows} rows
            </div>
            <div className="flex items-center gap-2">
              <span>Rows per page:</span>
              <select
                value={pageSize}
                className="border rounded px-2 py-1"
                disabled
              >
                <option value={100}>100</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      {/* Insert/Edit Modal */}
      {showInsertModal && selectedTable && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card className="w-full max-w-2xl max-h-[80vh] overflow-y-auto">
            <CardHeader>
              <CardTitle>
                {editingRow ? 'Edit Row' : 'Insert New Row'}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {selectedTable.columns.map(column => (
                  <div key={column.name}>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      {column.name}
                      {!column.nullable && <span className="text-red-500 ml-1">*</span>}
                      {column.is_primary_key && <span className="text-yellow-600 ml-1">🔑</span>}
                    </label>
                    <input
                      type="text"
                      defaultValue={editingRow?.[column.name] || ''}
                      placeholder={column.type}
                      className="w-full px-3 py-2 border rounded"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Type: {column.type} {column.nullable && '(nullable)'}
                    </p>
                  </div>
                ))}
                
                <div className="flex gap-2 pt-4">
                  <Button onClick={() => {
                    // TODO: Implement save
                    alert('Save functionality coming soon');
                    setShowInsertModal(false);
                  }}>
                    {editingRow ? 'Update' : 'Insert'}
                  </Button>
                  <Button variant="secondary" onClick={() => setShowInsertModal(false)}>
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
