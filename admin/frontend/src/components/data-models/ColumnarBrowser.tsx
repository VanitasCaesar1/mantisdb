import React, { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';

interface Table {
  name: string;
  columns: Array<{
    name: string;
    data_type: string;
    nullable: boolean;
    indexed: boolean;
    primary_key: boolean;
  }>;
  rows: Array<{
    values: Record<string, any>;
    row_id: number;
    version: number;
  }>;
}

export const ColumnarBrowser: React.FC = () => {
  const [tables, setTables] = useState<Array<{name: string; columns: number; rows: number}>>([]);
  const [selectedTable, setSelectedTable] = useState<string | null>(null);
  const [tableData, setTableData] = useState<Table | null>(null);
  const [rows, setRows] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [_showCreateTable, _setShowCreateTable] = useState(false);
  const [_showInsertRow, _setShowInsertRow] = useState(false);
  const [showCQLModal, setShowCQLModal] = useState(false);
  const [cqlStatement, setCqlStatement] = useState('');

  useEffect(() => {
    loadTables();
  }, []);

  useEffect(() => {
    if (selectedTable) {
      loadTable(selectedTable);
      queryRows(selectedTable);
    }
  }, [selectedTable]);

  const loadTables = async () => {
    try {
      const response = await fetch('http://localhost:8081/api/columnar/tables');
      const data = await response.json();
      
      if (data.success && data.tables) {
        setTables(data.tables);
      }
    } catch (error) {
      console.error('Failed to load tables:', error);
    }
  };

  const loadTable = async (tableName: string) => {
    try {
      const response = await fetch(`http://localhost:8081/api/columnar/tables/${tableName}`);
      const data = await response.json();
      
      if (data.success && data.table) {
        setTableData(data.table);
      }
    } catch (error) {
      console.error('Failed to load table:', error);
    }
  };

  const queryRows = async (tableName: string, filters?: any[]) => {
    setLoading(true);
    try {
      const body: any = { limit: 100 };
      if (filters) body.filters = filters;

      const response = await fetch(`http://localhost:8081/api/columnar/tables/${tableName}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      
      const data = await response.json();
      
      if (data.success && data.rows) {
        setRows(data.rows);
      }
    } catch (error) {
      console.error('Failed to query rows:', error);
    } finally {
      setLoading(false);
    }
  };

  const executeCQL = async () => {
    try {
      const response = await fetch('http://localhost:8081/api/columnar/cql', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ statement: cqlStatement }),
      });

      const data = await response.json();
      
      if (data.success) {
        alert('CQL executed successfully!');
        setShowCQLModal(false);
        loadTables();
        if (selectedTable) {
          queryRows(selectedTable);
        }
      } else {
        alert('CQL execution failed: ' + data.error);
      }
    } catch (error) {
      console.error('Failed to execute CQL:', error);
      alert('Failed to execute CQL');
    }
  };

  const createIndex = async () => {
    if (!selectedTable || !tableData) return;

    const columnName = prompt('Enter column name to index:');
    if (!columnName) return;

    try {
      const response = await fetch(
        `http://localhost:8081/api/columnar/tables/${selectedTable}/indexes`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: `idx_${columnName}`,
            columns: [columnName],
            unique: false,
            index_type: 'btree',
          }),
        }
      );

      const data = await response.json();
      
      if (data.success) {
        alert('Index created successfully!');
        loadTable(selectedTable);
      }
    } catch (error) {
      console.error('Failed to create index:', error);
      alert('Failed to create index');
    }
  };

  return (
    <div className="space-y-4">
      {/* Tables Bar */}
      <Card>
        <CardHeader>
          <div className="flex justify-between items-center">
            <CardTitle>Columnar Tables (Cassandra-style)</CardTitle>
            <div className="flex gap-2">
              <Button variant="secondary" onClick={() => setShowCQLModal(true)}>
                Execute CQL
              </Button>
              <Button variant="secondary" onClick={createIndex} disabled={!selectedTable}>
                Create Index
              </Button>
              <Button onClick={() => alert('Insert row feature coming soon')} disabled={!selectedTable}>
                Insert Row
              </Button>
              <Button onClick={() => alert('Create table feature coming soon')}>
                Create Table
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            {tables.map(table => (
              <button
                key={table.name}
                onClick={() => setSelectedTable(table.name)}
                className={`px-4 py-2 rounded ${
                  selectedTable === table.name
                    ? 'bg-mantis-600 text-white'
                    : 'bg-gray-100 hover:bg-gray-200'
                }`}
              >
                {table.name} ({table.rows} rows, {table.columns} cols)
              </button>
            ))}
            {tables.length === 0 && (
              <div className="text-gray-500">No tables found. Create one to get started.</div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Table Schema */}
      {tableData && (
        <Card>
          <CardHeader>
            <CardTitle>Schema: {tableData.name}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Column</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Type</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Nullable</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Indexed</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Primary Key</th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {tableData.columns.map(col => (
                    <tr key={col.name}>
                      <td className="px-6 py-3 whitespace-nowrap font-medium">{col.name}</td>
                      <td className="px-6 py-3 whitespace-nowrap text-sm text-gray-600">{col.data_type}</td>
                      <td className="px-6 py-3 whitespace-nowrap text-sm">
                        {col.nullable ? '✓' : '✗'}
                      </td>
                      <td className="px-6 py-3 whitespace-nowrap text-sm">
                        {col.indexed ? '✓' : '✗'}
                      </td>
                      <td className="px-6 py-3 whitespace-nowrap text-sm">
                        {col.primary_key ? '✓' : '✗'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Table Data */}
      {selectedTable && (
        <Card>
          <CardHeader>
            <CardTitle>Data: {selectedTable}</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="text-center py-4">Loading...</div>
            ) : rows.length === 0 ? (
              <div className="text-center py-4 text-gray-500">No rows found</div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      {Object.keys(rows[0]).map(col => (
                        <th key={col} className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                          {col}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {rows.map((row, idx) => (
                      <tr key={idx} className="hover:bg-gray-50">
                        {Object.entries(row).map(([col, val]) => (
                          <td key={col} className="px-6 py-3 whitespace-nowrap text-sm">
                            {JSON.stringify(val)}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* CQL Modal */}
      {showCQLModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card className="w-full max-w-3xl">
            <CardHeader>
              <CardTitle>Execute CQL (Cassandra Query Language)</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">CQL Statement</label>
                  <textarea
                    value={cqlStatement}
                    onChange={(e) => setCqlStatement(e.target.value)}
                    className="w-full px-3 py-2 border rounded font-mono"
                    rows={10}
                    placeholder={`CREATE TABLE users (
  id UUID PRIMARY KEY,
  name TEXT,
  email TEXT,
  created_at TIMESTAMP
);

SELECT * FROM users WHERE id = ?;
INSERT INTO users (id, name, email) VALUES (?, ?, ?);`}
                  />
                  <div className="text-xs text-gray-500 mt-2">
                    Supported: CREATE TABLE, SELECT, INSERT, UPDATE, DELETE
                  </div>
                </div>
                
                <div className="flex gap-2">
                  <Button onClick={executeCQL}>
                    Execute
                  </Button>
                  <Button variant="secondary" onClick={() => setShowCQLModal(false)}>
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
