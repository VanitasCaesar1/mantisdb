import { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';
import { DatabaseIcon } from '../icons';
import { useTables, useTableData } from '../../hooks/useApi';

export function DataBrowserSection() {
  const [selectedTable, setSelectedTable] = useState<string | null>(null);
  const [selectedTableType, setSelectedTableType] = useState<string>('collection');
  const [currentPage, setCurrentPage] = useState(0);
  const [pageSize] = useState(20);

  const { data: tablesData, loading: tablesLoading, error: tablesError } = useTables();
  const { data: tableData, loading: tableDataLoading, error: tableDataError } = useTableData(
    selectedTable || '',
    { 
      limit: pageSize, 
      offset: currentPage * pageSize,
      type: selectedTableType
    }
  );

  const tables = tablesData?.tables || [];

  const selectTable = (name: string, type: string) => {
    setSelectedTable(name);
    setSelectedTableType(type);
    setCurrentPage(0);
  };

  if (tablesError) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Data Browser</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-12">
            <div className="text-red-400 mb-4">
              <DatabaseIcon className="w-12 h-12 mx-auto" />
            </div>
            <h3 className="text-lg font-medium text-gray-900 mb-2">Connection Error</h3>
            <p className="text-red-600 mb-4">{tablesError}</p>
            <Button variant="secondary" onClick={() => window.location.reload()}>
              Retry Connection
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {/* Tables List */}
      <Card>
        <CardHeader>
          <CardTitle>Tables & Collections</CardTitle>
        </CardHeader>
        <CardContent>
          {tablesLoading ? (
            <div className="text-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-mantis-600 mx-auto"></div>
              <p className="text-gray-600 mt-2">Loading tables...</p>
            </div>
          ) : tables.length === 0 ? (
            <div className="text-center py-8">
              <DatabaseIcon className="w-12 h-12 mx-auto text-gray-400 mb-4" />
              <h3 className="text-lg font-medium text-gray-900 mb-2">No Tables Found</h3>
              <p className="text-gray-600">Create some data to see tables here.</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {tables.map((table) => (
                <div
                  key={table.name}
                  className={`p-4 border rounded-lg cursor-pointer transition-colors ${
                    selectedTable === table.name
                      ? 'border-mantis-500 bg-mantis-50'
                      : 'border-gray-200 hover:border-gray-300'
                  }`}
                  onClick={() => selectTable(table.name, table.type)}
                >
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="font-medium text-gray-900">{table.name}</h4>
                    <span className={`px-2 py-1 text-xs rounded-full ${
                      table.type === 'table' ? 'bg-blue-100 text-blue-800' :
                      table.type === 'collection' ? 'bg-green-100 text-green-800' :
                      'bg-purple-100 text-purple-800'
                    }`}>
                      {table.type}
                    </span>
                  </div>
                  <div className="text-sm text-gray-600 space-y-1">
                    <div className="flex justify-between">
                      <span>Rows:</span>
                      <span>{table.row_count?.toLocaleString() || 0}</span>
                    </div>
                    <div className="flex justify-between">
                      <span>Size:</span>
                      <span>{formatBytes(table.size_bytes || 0)}</span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Table Data */}
      {selectedTable && (
        <Card>
          <CardHeader>
            <CardTitle>Data: {selectedTable}</CardTitle>
          </CardHeader>
          <CardContent>
            {tableDataLoading ? (
              <div className="text-center py-8">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-mantis-600 mx-auto"></div>
                <p className="text-gray-600 mt-2">Loading data...</p>
              </div>
            ) : tableDataError ? (
              <div className="text-center py-8">
                <p className="text-red-600 mb-4">{tableDataError}</p>
                <Button variant="secondary" onClick={() => setSelectedTable(null)}>
                  Back to Tables
                </Button>
              </div>
            ) : (
              <div className="space-y-4">
                {/* Data Table */}
                <div className="overflow-x-auto">
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                      <tr>
                        {tableData?.data && tableData.data.length > 0 && 
                          Object.keys(tableData.data[0]).map((column) => (
                            <th
                              key={column}
                              className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                            >
                              {column}
                            </th>
                          ))
                        }
                      </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                      {tableData?.data?.map((row: any, index: number) => (
                        <tr key={index} className="hover:bg-gray-50">
                          {Object.values(row).map((value: any, cellIndex: number) => (
                            <td
                              key={cellIndex}
                              className="px-6 py-4 whitespace-nowrap text-sm text-gray-900"
                            >
                              {formatCellValue(value)}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {/* Pagination */}
                {tableData && (
                  <div className="flex items-center justify-between">
                    <div className="text-sm text-gray-700">
                      Showing {currentPage * pageSize + 1} to{' '}
                      {Math.min((currentPage + 1) * pageSize, tableData.total_count || 0)} of{' '}
                      {tableData.total_count || 0} results
                    </div>
                    <div className="flex space-x-2">
                      <Button
                        variant="secondary"
                        disabled={currentPage === 0}
                        onClick={() => setCurrentPage(currentPage - 1)}
                      >
                        Previous
                      </Button>
                      <Button
                        variant="secondary"
                        disabled={!tableData.total_count || (currentPage + 1) * pageSize >= tableData.total_count}
                        onClick={() => setCurrentPage(currentPage + 1)}
                      >
                        Next
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatCellValue(value: any): string {
  if (value === null || value === undefined) return '';
  if (typeof value === 'object') return JSON.stringify(value);
  if (typeof value === 'boolean') return value ? 'true' : 'false';
  return String(value);
}