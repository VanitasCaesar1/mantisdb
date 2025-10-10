import { useState, useEffect } from 'react';
import { Database, Table2, Search, Plus, Columns, Link } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '../ui';
import { apiClient } from '../../api/client';
import { CreateTableModal } from '../table-editor/CreateTableModal';

interface TableInfo {
  name: string;
  type?: string;
  columns?: any[];
  row_count?: number;
  size_bytes?: number;
}

interface Column {
  name: string;
  type: string;
}

export const SchemaVisualizerSection = () => {
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [selectedTable, setSelectedTable] = useState<string | null>(null);
  const [tableDetails, setTableDetails] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [showCreateModal, setShowCreateModal] = useState(false);

  useEffect(() => {
    loadTables();
  }, []);

  useEffect(() => {
    if (selectedTable) {
      loadTableDetails(selectedTable);
    }
  }, [selectedTable]);

  const loadTables = async () => {
    try {
      setLoading(true);
      const response = await apiClient.getTables();
      if (response.success && response.data?.tables) {
        setTables(response.data.tables);
      }
    } catch (error) {
      console.error('Failed to load tables:', error);
    } finally {
      setLoading(false);
    }
  };

  const loadTableDetails = async (tableName: string) => {
    try {
      const response = await apiClient.getTableData(tableName, { limit: 1 });
      if (response.success && response.data) {
        setTableDetails(response.data);
      }
    } catch (error) {
      console.error('Failed to load table details:', error);
    }
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

  const filteredTables = tables.filter(table =>
    table.name.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const getTypeColor = (type: string) => {
    const typeMap: Record<string, string> = {
      'string': 'text-blue-600 bg-blue-50',
      'int': 'text-green-600 bg-green-50',
      'float': 'text-green-600 bg-green-50',
      'bool': 'text-purple-600 bg-purple-50',
      'timestamp': 'text-yellow-600 bg-yellow-50',
      'json': 'text-orange-600 bg-orange-50',
    };
    return typeMap[type] || 'text-gray-600 bg-gray-50';
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-mantis-600 mx-auto"></div>
          <p className="text-gray-600 mt-4">Loading schema...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Database Schema</h1>
          <p className="text-gray-600 mt-1">Explore your database structure and relationships</p>
        </div>
        <button 
          onClick={() => setShowCreateModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors"
        >
          <Plus className="w-4 h-4" />
          Create Table
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-mantis-100 rounded-lg">
                <Database className="w-5 h-5 text-mantis-600" />
              </div>
              <div>
                <p className="text-sm text-gray-600">Total Tables</p>
                <p className="text-xl font-bold text-gray-900">{tables.length}</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-blue-100 rounded-lg">
                <Columns className="w-5 h-5 text-blue-600" />
              </div>
              <div>
                <p className="text-sm text-gray-600">Total Columns</p>
                <p className="text-xl font-bold text-gray-900">
                  {tables.reduce((sum, t) => sum + (t.columns?.length || 0), 0)}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-green-100 rounded-lg">
                <Table2 className="w-5 h-5 text-green-600" />
              </div>
              <div>
                <p className="text-sm text-gray-600">Total Rows</p>
                <p className="text-xl font-bold text-gray-900">
                  {tables.reduce((sum, t) => sum + (t.row_count || 0), 0)}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-yellow-100 rounded-lg">
                <Link className="w-5 h-5 text-yellow-600" />
              </div>
              <div>
                <p className="text-sm text-gray-600">Relationships</p>
                <p className="text-xl font-bold text-gray-900">0</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Search and View Toggle */}
      <div className="flex items-center justify-between gap-4">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search tables..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-500"
          />
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setViewMode('grid')}
            className={`px-3 py-2 rounded-lg transition-colors ${
              viewMode === 'grid'
                ? 'bg-mantis-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            Grid
          </button>
          <button
            onClick={() => setViewMode('list')}
            className={`px-3 py-2 rounded-lg transition-colors ${
              viewMode === 'list'
                ? 'bg-mantis-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            List
          </button>
        </div>
      </div>

      {/* Tables Display */}
      {filteredTables.length === 0 ? (
        <Card>
          <CardContent className="p-12">
            <div className="text-center">
              <Database className="w-16 h-16 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-medium text-gray-900 mb-2">No tables found</h3>
              <p className="text-gray-600 mb-4">
                {searchTerm ? 'Try a different search term' : 'Create your first table to get started'}
              </p>
              {!searchTerm && (
                <button className="px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors">
                  Create Table
                </button>
              )}
            </div>
          </CardContent>
        </Card>
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredTables.map((table) => (
            <Card
              key={table.name}
              className={`cursor-pointer hover:shadow-lg transition-all ${
                selectedTable === table.name ? 'ring-2 ring-mantis-500' : ''
              }`}
              onClick={() => setSelectedTable(table.name)}
            >
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Table2 className="w-5 h-5 text-mantis-600" />
                    <CardTitle className="text-base">{table.name}</CardTitle>
                  </div>
                  {table.row_count !== undefined && (
                    <span className="text-xs text-gray-500">{table.row_count} rows</span>
                  )}
                </div>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {table.columns && table.columns.length > 0 ? (
                    <>
                      {table.columns.slice(0, 5).map((column, idx) => (
                        <div key={idx} className="flex items-center justify-between text-sm">
                          <span className="text-gray-700">{column}</span>
                        </div>
                      ))}
                      {table.columns.length > 5 && (
                        <div className="text-xs text-gray-500 pt-1">
                          +{table.columns.length - 5} more columns
                        </div>
                      )}
                    </>
                  ) : (
                    <p className="text-sm text-gray-500">No column information</p>
                  )}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : (
        <Card>
          <CardContent className="p-0">
            <table className="w-full">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Table Name
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Columns
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Rows
                  </th>
                  <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {filteredTables.map((table) => (
                  <tr
                    key={table.name}
                    className={`hover:bg-gray-50 cursor-pointer ${
                      selectedTable === table.name ? 'bg-mantis-50' : ''
                    }`}
                    onClick={() => setSelectedTable(table.name)}
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center gap-2">
                        <Table2 className="w-4 h-4 text-mantis-600" />
                        <span className="font-medium text-gray-900">{table.name}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {table.columns?.length || 0} columns
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {table.row_count || 0}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <button className="text-mantis-600 hover:text-mantis-900">
                        View Details
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      )}

      {/* Table Details Panel */}
      {selectedTable && tableDetails && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Table2 className="w-5 h-5 text-mantis-600" />
                <CardTitle>{selectedTable}</CardTitle>
              </div>
              <button
                onClick={() => setSelectedTable(null)}
                className="text-gray-400 hover:text-gray-600"
              >
                ✕
              </button>
            </div>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div>
                <h3 className="text-sm font-medium text-gray-700 mb-2">Table Information</h3>
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <span className="text-gray-600">Name:</span>
                    <span className="ml-2 font-medium">{tableDetails.name}</span>
                  </div>
                  <div>
                    <span className="text-gray-600">Columns:</span>
                    <span className="ml-2 font-medium">{tableDetails.columns?.length || 0}</span>
                  </div>
                </div>
              </div>

              {tableDetails.columns && tableDetails.columns.length > 0 && (
                <div>
                  <h3 className="text-sm font-medium text-gray-700 mb-2">Columns</h3>
                  <div className="border border-gray-200 rounded-lg overflow-hidden">
                    <table className="w-full text-sm">
                      <thead className="bg-gray-50">
                        <tr>
                          <th className="px-4 py-2 text-left font-medium text-gray-600">Name</th>
                          <th className="px-4 py-2 text-left font-medium text-gray-600">Type</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-gray-200">
                        {tableDetails.columns.map((col: Column, idx: number) => (
                          <tr key={idx} className="hover:bg-gray-50">
                            <td className="px-4 py-2 font-medium text-gray-900">{col.name}</td>
                            <td className="px-4 py-2">
                              <span className={`px-2 py-1 text-xs font-medium rounded ${getTypeColor(col.type)}`}>
                                {col.type}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Create Table Modal */}
      <CreateTableModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSubmit={handleCreateTable}
      />
    </div>
  );
};
