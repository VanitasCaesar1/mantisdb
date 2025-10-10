import React, { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';

interface Column {
  name: string;
  type: string;
  nullable?: boolean;
  primary?: boolean;
}

interface TableGridProps {
  tableName: string;
  onRefresh?: () => void;
}

export const TableGrid: React.FC<TableGridProps> = ({ tableName }) => {
  const [data, setData] = useState<any[]>([]);
  const [columns, setColumns] = useState<Column[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(50);
  const [totalCount, setTotalCount] = useState(0);
  const [selectedRows, setSelectedRows] = useState<Set<number>>(new Set());
  const [editingCell, setEditingCell] = useState<{row: number, col: string} | null>(null);
  const [editValue, setEditValue] = useState<any>('');

  useEffect(() => {
    loadData();
  }, [tableName, page]);

  const loadData = async () => {
    try {
      setLoading(true);
      const offset = (page - 1) * pageSize;
      const response = await fetch(
        `http://localhost:8081/api/tables/${tableName}?limit=${pageSize}&offset=${offset}`
      );
      
      if (!response.ok) throw new Error('Failed to load data');
      
      const result = await response.json();
      setData(result.data || []);
      setTotalCount(result.total_count || 0);
      
      // Extract columns from first row or use schema
      if (result.data && result.data.length > 0) {
        const firstRow = result.data[0];
        const cols: Column[] = Object.keys(firstRow).map(key => ({
          name: key,
          type: typeof firstRow[key],
        }));
        setColumns(cols);
      }
    } catch (error) {
      console.error('Error loading table data:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCellClick = (rowIndex: number, colName: string) => {
    setEditingCell({ row: rowIndex, col: colName });
    setEditValue(data[rowIndex][colName]);
  };

  const handleCellSave = async () => {
    if (!editingCell) return;

    try {
      const row = data[editingCell.row];
      const updatedRow = { ...row, [editingCell.col]: editValue };
      
      // Assuming rows have an 'id' field
      if (row.id) {
        const response = await fetch(
          `http://localhost:8081/api/tables/${tableName}/data/${row.id}`,
          {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(updatedRow),
          }
        );

        if (response.ok) {
          const newData = [...data];
          newData[editingCell.row] = updatedRow;
          setData(newData);
        }
      }
    } catch (error) {
      console.error('Error updating cell:', error);
    } finally {
      setEditingCell(null);
    }
  };

  const handleRowSelect = (rowIndex: number) => {
    const newSelection = new Set(selectedRows);
    if (newSelection.has(rowIndex)) {
      newSelection.delete(rowIndex);
    } else {
      newSelection.add(rowIndex);
    }
    setSelectedRows(newSelection);
  };

  const handleDeleteSelected = async () => {
    if (selectedRows.size === 0) return;
    
    if (!confirm(`Delete ${selectedRows.size} row(s)?`)) return;

    try {
      for (const rowIndex of selectedRows) {
        const row = data[rowIndex];
        if (row.id) {
          await fetch(
            `http://localhost:8081/api/tables/${tableName}/data/${row.id}`,
            { method: 'DELETE' }
          );
        }
      }
      setSelectedRows(new Set());
      loadData();
    } catch (error) {
      console.error('Error deleting rows:', error);
    }
  };

  const handleAddRow = async () => {
    const newRow: any = {};
    columns.forEach(col => {
      newRow[col.name] = null;
    });

    try {
      const response = await fetch(
        `http://localhost:8081/api/tables/${tableName}/data`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(newRow),
        }
      );

      if (response.ok) {
        loadData();
      }
    } catch (error) {
      console.error('Error adding row:', error);
    }
  };

  const totalPages = Math.ceil(totalCount / pageSize);

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
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle>{tableName}</CardTitle>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={handleAddRow}>
            + Add Row
          </Button>
          {selectedRows.size > 0 && (
            <Button variant="danger" onClick={handleDeleteSelected}>
              Delete ({selectedRows.size})
            </Button>
          )}
          <Button variant="secondary" onClick={loadData}>
            Refresh
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-3 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  <input
                    type="checkbox"
                    onChange={(e) => {
                      if (e.target.checked) {
                        setSelectedRows(new Set(data.map((_, i) => i)));
                      } else {
                        setSelectedRows(new Set());
                      }
                    }}
                  />
                </th>
                {columns.map((col) => (
                  <th
                    key={col.name}
                    className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                  >
                    <div className="flex items-center gap-2">
                      {col.name}
                      <span className="text-xs text-gray-400">({col.type})</span>
                    </div>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {data.map((row, rowIndex) => (
                <tr
                  key={rowIndex}
                  className={selectedRows.has(rowIndex) ? 'bg-blue-50' : 'hover:bg-gray-50'}
                >
                  <td className="px-3 py-2">
                    <input
                      type="checkbox"
                      checked={selectedRows.has(rowIndex)}
                      onChange={() => handleRowSelect(rowIndex)}
                    />
                  </td>
                  {columns.map((col) => (
                    <td
                      key={col.name}
                      className="px-6 py-2 whitespace-nowrap text-sm text-gray-900 cursor-pointer"
                      onClick={() => handleCellClick(rowIndex, col.name)}
                    >
                      {editingCell?.row === rowIndex && editingCell?.col === col.name ? (
                        <input
                          type="text"
                          value={editValue}
                          onChange={(e) => setEditValue(e.target.value)}
                          onBlur={handleCellSave}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') handleCellSave();
                            if (e.key === 'Escape') setEditingCell(null);
                          }}
                          autoFocus
                          className="w-full px-2 py-1 border rounded"
                        />
                      ) : (
                        <span>{JSON.stringify(row[col.name])}</span>
                      )}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        <div className="mt-4 flex items-center justify-between">
          <div className="text-sm text-gray-700">
            Showing {(page - 1) * pageSize + 1} to {Math.min(page * pageSize, totalCount)} of {totalCount} results
          </div>
          <div className="flex gap-2">
            <Button
              variant="secondary"
              disabled={page === 1}
              onClick={() => setPage(p => Math.max(1, p - 1))}
            >
              Previous
            </Button>
            <span className="px-4 py-2 text-sm">
              Page {page} of {totalPages}
            </span>
            <Button
              variant="secondary"
              disabled={page >= totalPages}
              onClick={() => setPage(p => Math.min(totalPages, p + 1))}
            >
              Next
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
};
