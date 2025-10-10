import { useState, useEffect } from 'react';
import { Database, Table2, Plus, Trash2, Edit, Search, ChevronLeft, ChevronRight, RefreshCw } from 'lucide-react';
import { apiClient } from '../../api/client';

interface Table {
  name: string;
  row_count?: number;
}

interface Row {
  [key: string]: any;
}

export const DataBrowser = () => {
  const [tables, setTables] = useState<Table[]>([]);
  const [selectedTable, setSelectedTable] = useState<string>('');
  const [rows, setRows] = useState<Row[]>([]);
  const [columns, setColumns] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>('');
  const [searchTerm, setSearchTerm] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize] = useState(50);

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
      const response = await apiClient.getColumnarTables();
      if (response.success && response.data?.tables) {
        const tableList = response.data.tables;
        setTables(tableList);
        if (tableList.length > 0 && !selectedTable) {
          setSelectedTable(tableList[0].name);
        }
      } else {
        setError(response.error || 'No tables found. Create a table to get started.');
        setTables([]);
      }
    } catch (err) {
      console.error('Failed to load tables:', err);
      setError('Failed to connect to database. Please check if MantisDB is running.');
      setTables([]);
    }
  };

  const loadTableData = async () => {
    if (!selectedTable) return;
    
    setLoading(true);
    setError('');
    
    try {
      const response = await apiClient.queryColumnarTable(selectedTable, {
        limit: pageSize,
        offset: (page - 1) * pageSize,
      });

      if (response.success && response.data?.rows) {
        const rowData = response.data.rows;
        setRows(rowData);
        if (rowData.length > 0) {
          setColumns(Object.keys(rowData[0]));
        } else {
          setColumns([]);
        }
      } else {
        setError(response.error || 'Failed to load table data');
        setRows([]);
        setColumns([]);
      }
    } catch (err) {
      console.error('Failed to load table data:', err);
      setError('Failed to connect to database. Please check if MantisDB is running.');
      setRows([]);
      setColumns([]);
    } finally {
      setLoading(false);
    }
  };

  const handleInsertRow = async () => {
    if (!selectedTable) return;
    
    // Create a new row with empty values for each column
    const newRow: Record<string, any> = {};
    columns.forEach(col => {
      newRow[col] = '';
    });
    
    try {
      const response = await apiClient.insertColumnarRows(selectedTable, [newRow]);
      
      if (response.success) {
        // Reload table data
        await loadTableData();
      } else {
        setError(response.error || 'Failed to insert row');
      }
    } catch (err) {
      console.error('Failed to insert row:', err);
      setError('Failed to insert row');
    }
  };
  
  const handleDeleteRow = async (rowIndex: number) => {
    if (!selectedTable || !confirm('Are you sure you want to delete this row?')) return;
    
    const row = rows[rowIndex];
    const rowId = row.id || rowIndex;
    
    try {
      const response = await apiClient.deleteColumnarRows(selectedTable, { where: { id: rowId } });
      
      if (response.success) {
        // Reload table data
        await loadTableData();
      } else {
        setError(response.error || 'Failed to delete row');
      }
    } catch (err) {
      console.error('Failed to delete row:', err);
      setError('Failed to delete row');
    }
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

  return (
    <div className="flex h-full bg-[#1a1a1a]">
      {/* Sidebar */}
      <div className="w-64 bg-[#0f0f0f] border-r border-gray-800 flex flex-col">
        <div className="p-4 border-b border-gray-800">
          <div className="flex items-center gap-2 text-white mb-2">
            <Database className="w-5 h-5" />
            <h2 className="font-semibold">Tables</h2>
          </div>
          <p className="text-sm text-gray-400">{tables.length} tables</p>
        </div>
        
        <div className="flex-1 overflow-y-auto p-2">
          {tables.map((table) => (
            <button
              key={table.name}
              onClick={() => {
                setSelectedTable(table.name);
                setPage(1);
              }}
              className={`w-full text-left px-3 py-2 rounded-lg mb-1 transition-all ${
                selectedTable === table.name
                  ? 'bg-mantis-600 text-white'
                  : 'text-gray-300 hover:bg-gray-800'
              }`}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Table2 className="w-4 h-4" />
                  <span className="font-medium">{table.name}</span>
                </div>
                {table.row_count !== undefined && (
                  <span className="text-xs text-gray-500">{table.row_count}</span>
                )}
              </div>
            </button>
          ))}
        </div>

        <div className="p-4 border-t border-gray-800">
          <button
            onClick={loadTables}
            className="w-full flex items-center justify-center gap-2 px-4 py-2 bg-gray-800 hover:bg-gray-700 text-white rounded-lg transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 flex flex-col">
        {/* Header */}
        <div className="bg-[#0f0f0f] border-b border-gray-800 px-6 py-4">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold text-white flex items-center gap-2">
                <Table2 className="w-6 h-6" />
                {selectedTable || 'Select a table'}
              </h1>
              <p className="text-sm text-gray-400 mt-1">
                {filteredRows.length} rows
              </p>
            </div>
            <div className="flex gap-2">
              <button 
                onClick={handleInsertRow}
                className="flex items-center gap-2 px-4 py-2 bg-mantis-600 hover:bg-mantis-700 text-white rounded-lg transition-colors"
              >
                <Plus className="w-4 h-4" />
                Insert Row
              </button>
            </div>
          </div>
        </div>

        {/* Toolbar */}
        <div className="bg-[#0f0f0f] border-b border-gray-800 px-6 py-3">
          <div className="flex items-center gap-4">
            <div className="flex-1 relative">
              <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-500" />
              <input
                type="text"
                placeholder="Search in table..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="w-full pl-10 pr-4 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-mantis-600"
              />
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setPage(Math.max(1, page - 1))}
                disabled={page === 1}
                className="p-2 bg-gray-800 hover:bg-gray-700 text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <span className="text-sm text-gray-400 px-3">
                Page {page}
              </span>
              <button
                onClick={() => setPage(page + 1)}
                disabled={filteredRows.length < pageSize}
                className="p-2 bg-gray-800 hover:bg-gray-700 text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        {/* Table Content */}
        <div className="flex-1 overflow-auto">
          {error && (
            <div className="m-6 p-4 bg-red-900/20 border border-red-800 rounded-lg text-red-400">
              {error}
            </div>
          )}

          {loading ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-gray-400">Loading...</div>
            </div>
          ) : !selectedTable ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-center">
                <Database className="w-16 h-16 text-gray-600 mx-auto mb-4" />
                <p className="text-gray-400 text-lg">Select a table to view data</p>
              </div>
            </div>
          ) : filteredRows.length === 0 ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-center">
                <Table2 className="w-16 h-16 text-gray-600 mx-auto mb-4" />
                <p className="text-gray-400 text-lg mb-2">No rows found</p>
                <button 
                  onClick={handleInsertRow}
                  className="flex items-center gap-2 px-4 py-2 bg-mantis-600 hover:bg-mantis-700 text-white rounded-lg transition-colors mx-auto"
                >
                  <Plus className="w-4 h-4" />
                  Insert First Row
                </button>
              </div>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead className="bg-[#0f0f0f] sticky top-0 z-10">
                  <tr>
                    {columns.map((column) => (
                      <th
                        key={column}
                        className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider border-b border-gray-800"
                      >
                        {column}
                      </th>
                    ))}
                    <th className="px-6 py-3 text-right text-xs font-medium text-gray-400 uppercase tracking-wider border-b border-gray-800">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-800">
                  {filteredRows.map((row, index) => (
                    <tr
                      key={index}
                      className="hover:bg-gray-900/50 transition-colors"
                    >
                      {columns.map((column) => (
                        <td key={column} className="px-6 py-4 text-sm text-gray-300">
                          <div className="max-w-xs truncate" title={formatValue(row[column])}>
                            {row[column] === null || row[column] === undefined ? (
                              <span className="text-gray-600 italic">NULL</span>
                            ) : typeof row[column] === 'object' ? (
                              <span className="text-mantis-400 font-mono text-xs">
                                {JSON.stringify(row[column])}
                              </span>
                            ) : (
                              formatValue(row[column])
                            )}
                          </div>
                        </td>
                      ))}
                      <td className="px-6 py-4 text-right text-sm">
                        <div className="flex items-center justify-end gap-2">
                          <button
                            className="p-1.5 text-gray-400 hover:text-mantis-400 hover:bg-gray-800 rounded transition-colors"
                            title="Edit"
                          >
                            <Edit className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handleDeleteRow(index)}
                            className="p-1.5 text-gray-400 hover:text-red-400 hover:bg-gray-800 rounded transition-colors"
                            title="Delete"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Footer */}
        {filteredRows.length > 0 && (
          <div className="bg-[#0f0f0f] border-t border-gray-800 px-6 py-3">
            <div className="flex items-center justify-between text-sm text-gray-400">
              <div>
                Showing {((page - 1) * pageSize) + 1} to {Math.min(page * pageSize, filteredRows.length)} of {filteredRows.length} rows
              </div>
              <div className="flex items-center gap-2">
                <span>Rows per page:</span>
                <select
                  value={pageSize}
                  className="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-white"
                  disabled
                >
                  <option value={50}>50</option>
                </select>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
