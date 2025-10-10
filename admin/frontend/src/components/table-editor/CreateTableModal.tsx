import { useState } from 'react';
import { X, Plus, Trash2 } from 'lucide-react';

interface Column {
  name: string;
  type: string;
  required: boolean;
}

interface CreateTableModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (name: string, type: string, columns: Column[]) => Promise<void>;
}

export const CreateTableModal = ({ isOpen, onClose, onSubmit }: CreateTableModalProps) => {
  const [tableName, setTableName] = useState('');
  const [tableType, setTableType] = useState('table');
  const [columns, setColumns] = useState<Column[]>([
    { name: 'id', type: 'integer', required: true },
  ]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const addColumn = () => {
    setColumns([...columns, { name: '', type: 'string', required: false }]);
  };

  const removeColumn = (index: number) => {
    setColumns(columns.filter((_, i) => i !== index));
  };

  const updateColumn = (index: number, field: keyof Column, value: any) => {
    const newColumns = [...columns];
    newColumns[index] = { ...newColumns[index], [field]: value };
    setColumns(newColumns);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!tableName.trim()) {
      setError('Table name is required');
      return;
    }

    if (columns.length === 0) {
      setError('At least one column is required');
      return;
    }

    if (columns.some(col => !col.name.trim())) {
      setError('All columns must have a name');
      return;
    }

    setLoading(true);
    try {
      await onSubmit(tableName, tableType, columns);
      // Reset form
      setTableName('');
      setTableType('table');
      setColumns([{ name: 'id', type: 'integer', required: true }]);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create table');
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b">
          <h2 className="text-2xl font-bold text-gray-900">Create New Table</h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {error && (
            <div className="p-4 bg-red-50 border border-red-200 rounded-lg text-red-700">
              {error}
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Table Name
            </label>
            <input
              type="text"
              value={tableName}
              onChange={(e) => setTableName(e.target.value)}
              className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-600"
              placeholder="e.g., users, products, orders"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Table Type
            </label>
            <select
              value={tableType}
              onChange={(e) => setTableType(e.target.value)}
              className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-600"
            >
              <option value="table">Table (Relational)</option>
              <option value="collection">Collection (Document)</option>
              <option value="keyvalue">Key-Value Store</option>
            </select>
          </div>

          <div>
            <div className="flex items-center justify-between mb-3">
              <label className="block text-sm font-medium text-gray-700">
                Columns
              </label>
              <button
                type="button"
                onClick={addColumn}
                className="flex items-center gap-2 px-3 py-1 text-sm bg-mantis-600 hover:bg-mantis-700 text-white rounded-lg transition-colors"
              >
                <Plus className="w-4 h-4" />
                Add Column
              </button>
            </div>

            <div className="space-y-3">
              {columns.map((column, index) => (
                <div key={index} className="flex items-center gap-3 p-3 bg-gray-50 rounded-lg">
                  <input
                    type="text"
                    value={column.name}
                    onChange={(e) => updateColumn(index, 'name', e.target.value)}
                    className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-600"
                    placeholder="Column name"
                    required
                  />
                  <select
                    value={column.type}
                    onChange={(e) => updateColumn(index, 'type', e.target.value)}
                    className="px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-600"
                  >
                    <option value="string">String</option>
                    <option value="integer">Integer</option>
                    <option value="float">Float</option>
                    <option value="boolean">Boolean</option>
                    <option value="date">Date</option>
                    <option value="json">JSON</option>
                  </select>
                  <label className="flex items-center gap-2 text-sm text-gray-700">
                    <input
                      type="checkbox"
                      checked={column.required}
                      onChange={(e) => updateColumn(index, 'required', e.target.checked)}
                      className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-600"
                    />
                    Required
                  </label>
                  {columns.length > 1 && (
                    <button
                      type="button"
                      onClick={() => removeColumn(index)}
                      className="p-2 text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
              ))}
            </div>
          </div>

          <div className="flex items-center justify-end gap-3 pt-4 border-t">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-6 py-2 bg-mantis-600 hover:bg-mantis-700 text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? 'Creating...' : 'Create Table'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
