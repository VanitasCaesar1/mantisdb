import React, { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '../ui';

interface Table {
  name: string;
  type: string;
  row_count?: number;
  columns?: Array<{
    name: string;
    type: string;
    nullable?: boolean;
    primary?: boolean;
  }>;
}

export const SchemaVisualizer: React.FC = () => {
  const [tables, setTables] = useState<Table[]>([]);
  const [selectedTable, setSelectedTable] = useState<Table | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadSchema();
  }, []);

  const loadSchema = async () => {
    try {
      const response = await fetch('http://localhost:8081/api/tables');
      const data = await response.json();
      
      if (data.tables) {
        setTables(data.tables);
      }
    } catch (error) {
      console.error('Error loading schema:', error);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <Card>
        <CardContent className="flex items-center justify-center h-64">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-mantis-600"></div>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
      {/* Tables List */}
      <Card>
        <CardHeader>
          <CardTitle>Tables ({tables.length})</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {tables.map((table) => (
              <div
                key={table.name}
                onClick={() => setSelectedTable(table)}
                className={`p-3 border rounded cursor-pointer transition-colors ${
                  selectedTable?.name === table.name
                    ? 'bg-mantis-50 border-mantis-500'
                    : 'hover:bg-gray-50'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div>
                    <h4 className="font-medium">{table.name}</h4>
                    <p className="text-xs text-gray-500">{table.type}</p>
                  </div>
                  {table.row_count !== undefined && (
                    <span className="text-xs text-gray-500">
                      {table.row_count} rows
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Table Details */}
      <div className="lg:col-span-2">
        {selectedTable ? (
          <Card>
            <CardHeader>
              <CardTitle>{selectedTable.name}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div className="flex gap-4 text-sm">
                  <div>
                    <span className="text-gray-600">Type:</span>{' '}
                    <span className="font-medium">{selectedTable.type}</span>
                  </div>
                  {selectedTable.row_count !== undefined && (
                    <div>
                      <span className="text-gray-600">Rows:</span>{' '}
                      <span className="font-medium">{selectedTable.row_count}</span>
                    </div>
                  )}
                </div>

                {selectedTable.columns && selectedTable.columns.length > 0 && (
                  <div>
                    <h4 className="font-medium mb-3">Columns</h4>
                    <div className="border rounded overflow-hidden">
                      <table className="min-w-full divide-y divide-gray-200">
                        <thead className="bg-gray-50">
                          <tr>
                            <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
                              Name
                            </th>
                            <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
                              Type
                            </th>
                            <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
                              Constraints
                            </th>
                          </tr>
                        </thead>
                        <tbody className="bg-white divide-y divide-gray-200">
                          {selectedTable.columns.map((column) => (
                            <tr key={column.name}>
                              <td className="px-4 py-3 text-sm font-medium text-gray-900">
                                {column.name}
                              </td>
                              <td className="px-4 py-3 text-sm text-gray-600">
                                {column.type}
                              </td>
                              <td className="px-4 py-3 text-sm">
                                <div className="flex gap-1">
                                  {column.primary && (
                                    <span className="px-2 py-1 text-xs bg-yellow-100 text-yellow-800 rounded">
                                      PRIMARY KEY
                                    </span>
                                  )}
                                  {!column.nullable && (
                                    <span className="px-2 py-1 text-xs bg-blue-100 text-blue-800 rounded">
                                      NOT NULL
                                    </span>
                                  )}
                                  {column.nullable && !column.primary && (
                                    <span className="px-2 py-1 text-xs bg-gray-100 text-gray-600 rounded">
                                      NULLABLE
                                    </span>
                                  )}
                                </div>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )}

                {/* Visual Schema Diagram */}
                <div className="border rounded p-6 bg-gray-50">
                  <h4 className="font-medium mb-4">Schema Diagram</h4>
                  <div className="bg-white border-2 border-gray-300 rounded p-4 shadow-sm">
                    <div className="bg-mantis-600 text-white px-3 py-2 rounded-t flex items-center justify-between">
                      <span className="font-medium">{selectedTable.name}</span>
                      <span className="text-xs opacity-75">{selectedTable.type}</span>
                    </div>
                    <div className="border-t-2 border-gray-300">
                      {selectedTable.columns?.map((column, idx) => (
                        <div
                          key={column.name}
                          className={`px-3 py-2 flex justify-between items-center ${
                            idx !== selectedTable.columns!.length - 1 ? 'border-b' : ''
                          }`}
                        >
                          <div className="flex items-center gap-2">
                            {column.primary && (
                              <svg className="w-4 h-4 text-yellow-500" fill="currentColor" viewBox="0 0 20 20">
                                <path d="M10 2a8 8 0 100 16 8 8 0 000-16zm-1 11V7h2v6h-2z"/>
                              </svg>
                            )}
                            <span className="font-mono text-sm">{column.name}</span>
                          </div>
                          <span className="text-xs text-gray-500">{column.type}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent className="text-center py-16 text-gray-500">
              Select a table to view its schema
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
};
