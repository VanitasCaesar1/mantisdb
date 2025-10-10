import { useEffect, useState } from 'react';
import { X, Plus, Trash2 } from 'lucide-react';
import { apiClient } from '../../api/client';

export interface ColumnDef {
  name: string;
  type: string;
  required: boolean;
}

interface ManageColumnsModalProps {
  isOpen: boolean;
  tableName: string;
  onClose: () => void;
  onSaved: () => void;
}

export function ManageColumnsModal({ isOpen, tableName, onClose, onSaved }: ManageColumnsModalProps) {
  const [columns, setColumns] = useState<ColumnDef[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!isOpen) return;
    (async () => {
      setError('');
      try {
        const res = await apiClient.getTableSchema(tableName);
        if (res.success) {
          setColumns((res.data?.columns as ColumnDef[]) || []);
        } else {
          setColumns([]);
        }
      } catch (e) {
        setColumns([]);
      }
    })();
  }, [isOpen, tableName]);

  const addColumn = () => {
    setColumns(prev => [...prev, { name: '', type: 'string', required: false }]);
  };

  const removeColumn = (index: number) => {
    setColumns(prev => prev.filter((_, i) => i !== index));
  };

  const updateColumn = (index: number, field: keyof ColumnDef, value: any) => {
    setColumns(prev => {
      const next = [...prev];
      next[index] = { ...next[index], [field]: value } as ColumnDef;
      return next;
    });
  };

  const handleSave = async () => {
    setError('');
    // Basic validation
    if (columns.some(c => !c.name.trim())) {
      setError('All columns must have a name');
      return;
    }
    setLoading(true);
    try {
      const res = await apiClient.updateTableSchema(tableName, columns);
      if (!res.success) {
        throw new Error(res.error || 'Failed to update schema');
      }
      onSaved();
      onClose();
    } catch (e: any) {
      setError(e?.message || 'Failed to update schema');
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b">
          <h2 className="text-2xl font-bold text-gray-900">Manage Columns — {tableName}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 transition-colors">
            <X className="w-6 h-6" />
          </button>
        </div>

        <div className="p-6 space-y-4">
          {error && (
            <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded">
              {error}
            </div>
          )}

          <div className="flex items-center justify-between">
            <div className="text-sm text-gray-600">Define the schema used to render and insert rows</div>
            <button
              onClick={addColumn}
              className="flex items-center gap-2 px-3 py-1 text-sm bg-mantis-600 hover:bg-mantis-700 text-white rounded-lg transition-colors"
            >
              <Plus className="w-4 h-4" /> Add Column
            </button>
          </div>

          <div className="space-y-3">
            {columns.length === 0 && (
              <div className="text-sm text-gray-500">No columns yet. Click "Add Column" to create one.</div>
            )}
            {columns.map((col, i) => (
              <div key={i} className="flex items-center gap-3 p-3 bg-gray-50 rounded-lg">
                <input
                  value={col.name}
                  onChange={e => updateColumn(i, 'name', e.target.value)}
                  placeholder="name"
                  className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-600"
                />
                <select
                  value={col.type}
                  onChange={e => updateColumn(i, 'type', e.target.value)}
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
                    checked={col.required}
                    onChange={e => updateColumn(i, 'required', e.target.checked)}
                    className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-600"
                  />
                  Required
                </label>
                <button
                  onClick={() => removeColumn(i)}
                  className="p-2 text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                  title="Remove column"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            ))}
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 pt-4 border-t p-6">
          <button onClick={onClose} className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg transition-colors">Cancel</button>
          <button
            onClick={handleSave}
            disabled={loading}
            className="px-6 py-2 bg-mantis-600 hover:bg-mantis-700 text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? 'Saving...' : 'Save Schema'}
          </button>
        </div>
      </div>
    </div>
  );
}
