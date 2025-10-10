import { useState, useEffect } from 'react';
import { Database, Plus, Trash2, RefreshCw, Search, Edit2, FileJson } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '../ui';

interface CollectionInfo {
  name: string;
  document_count: number;
}

interface DocItem {
  id: string;
  data: any;
}

export const DocumentDBSection = () => {
  const [collections, setCollections] = useState<CollectionInfo[]>([]);
  const [selectedCollection, setSelectedCollection] = useState<string>('');
  const [documents, setDocuments] = useState<DocItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>('');
  const [searchQuery, setSearchQuery] = useState('');
  const [showDocumentModal, setShowDocumentModal] = useState(false);
  const [editingDocument, setEditingDocument] = useState<DocItem | null>(null);
  const [newDocumentJson, setNewDocumentJson] = useState('{\n  \n}');
  const [modalMode, setModalMode] = useState<'json' | 'form'>('json');
  type FieldType = 'string' | 'number' | 'boolean' | 'date' | 'json';
  interface FormField { key: string; type: FieldType; value: string; }
  const [formFields, setFormFields] = useState<FormField[]>([]);

  useEffect(() => {
    loadCollections();
  }, []);

  useEffect(() => {
    if (selectedCollection) {
      loadDocuments();
    }
  }, [selectedCollection]);

  const loadCollections = async () => {
    try {
      setError('');
      const response = await fetch('/api/documents/collections');
      if (response.ok) {
        const data = await response.json();
        const items: CollectionInfo[] = (data.collections || []).map((c: any) => ({
          name: c.name,
          document_count: c.document_count ?? c.count ?? 0,
        }));
        setCollections(items);
        if (items.length > 0 && !selectedCollection) {
          setSelectedCollection(items[0].name);
        }
      }
    } catch (err) {
      console.error('Failed to load collections:', err);
      setError('Failed to load collections');
    }
  };

  const loadDocuments = async () => {
    if (!selectedCollection) return;
    
    setLoading(true);
    setError('');
    
    try {
      const response = await fetch(`/api/documents/${selectedCollection}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filter: {}, limit: 100 }),
      });

      if (response.ok) {
        const data = await response.json();
        const items: DocItem[] = (data.documents || []).map((d: any) => ({ id: d.id, data: d.data }));
        setDocuments(items);
      } else {
        setError('Failed to load documents');
      }
    } catch (err) {
      console.error('Failed to load documents:', err);
      setError('Failed to load documents');
    } finally {
      setLoading(false);
    }
  };

  const handleCreateDocument = async () => {
    if (!selectedCollection) return;
    
    try {
      const doc = modalMode === 'json'
        ? JSON.parse(newDocumentJson)
        : buildDocFromForm();
      let response: Response;
      if (editingDocument) {
        response = await fetch(`/api/documents/${selectedCollection}/${encodeURIComponent(editingDocument.id)}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ data: doc }),
        });
      } else {
        response = await fetch(`/api/documents/${selectedCollection}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ data: doc }),
        });
      }

      if (response.ok) {
        setShowDocumentModal(false);
        setNewDocumentJson('{\n  \n}');
        await loadDocuments();
      } else {
        setError('Failed to create document');
      }
    } catch (err) {
      setError('Invalid document format');
    }
  };

  const buildDocFromForm = () => {
    const obj: any = {};
    formFields.forEach(f => {
      let v: any = f.value;
      switch (f.type) {
        case 'number': v = Number(f.value); break;
        case 'boolean': v = f.value.toLowerCase() === 'true'; break;
        case 'date': v = new Date(f.value).toISOString(); break;
        case 'json':
          try { v = JSON.parse(f.value); } catch { throw new Error('Invalid JSON field: ' + f.key); }
          break;
        default: v = f.value;
      }
      obj[f.key] = v;
    });
    return obj;
  };

  const addFormField = () => setFormFields(prev => ([...prev, { key: '', type: 'string', value: '' }]));
  const removeFormField = (idx: number) => setFormFields(prev => prev.filter((_, i) => i !== idx));
  const updateFormField = (idx: number, patch: Partial<FormField>) => setFormFields(prev => prev.map((f, i) => i === idx ? { ...f, ...patch } : f));

  const handleDeleteDocument = async (id: string) => {
    if (!selectedCollection || !confirm('Delete this document?')) return;
    
    try {
      const response = await fetch(`/api/documents/${selectedCollection}/${id}`, {
        method: 'DELETE',
      });

      if (response.ok) {
        await loadDocuments();
      } else {
        setError('Failed to delete document');
      }
    } catch (err) {
      setError('Failed to delete document');
    }
  };

  const filteredDocuments = documents.filter(doc =>
    searchQuery === '' || JSON.stringify(doc).toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">Document Store</h2>
          <p className="text-sm text-gray-600 mt-1">MongoDB-style document database with flexible schemas</p>
        </div>
        <button
          onClick={async () => {
            const name = prompt('Enter collection name');
            if (!name) return;
            try {
              const resp = await fetch(`/api/documents/${encodeURIComponent(name)}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ data: {} }),
              });
              if (resp.ok) {
                await loadCollections();
                setSelectedCollection(name);
              } else {
                setError('Failed to create collection');
              }
            } catch (e) {
              setError('Failed to create collection');
            }
          }}
          className="flex items-center gap-2 px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Collection
        </button>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
          {error}
        </div>
      )}

      <div className="grid grid-cols-12 gap-6">
        {/* Collections Sidebar */}
        <div className="col-span-3">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Collections</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-1">
                {collections.map((collection) => (
                  <button
                    key={collection.name}
                    onClick={() => setSelectedCollection(collection.name)}
                    className={`w-full text-left px-3 py-2 rounded-lg transition-colors ${
                      selectedCollection === collection.name
                        ? 'bg-mantis-100 text-mantis-900 font-medium'
                        : 'hover:bg-gray-100 text-gray-700'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <Database className="w-4 h-4" />
                      <div className="flex-1 min-w-0">
                        <div className="text-sm truncate">{collection.name}</div>
                        <div className="text-xs text-gray-500">{collection.document_count} docs</div>
                      </div>
                    </div>
                  </button>
                ))}
                {collections.length === 0 && (
                  <div className="text-sm text-gray-500 text-center py-4">
                    No collections yet
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Documents View */}
        <div className="col-span-9">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>
                  {selectedCollection || 'Select a collection'}
                </CardTitle>
                <div className="flex items-center gap-2">
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                    <input
                      type="text"
                      placeholder="Search documents..."
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      className="pl-10 pr-4 py-2 border rounded-lg text-sm w-64"
                    />
                  </div>
                  <button
                    onClick={loadDocuments}
                    disabled={loading}
                    className="p-2 border rounded-lg hover:bg-gray-50 transition-colors"
                  >
                    <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
                  </button>
                  <button
                    onClick={() => {
                      setEditingDocument(null);
                      setNewDocumentJson('{\n  \n}');
                      setFormFields([]);
                      setModalMode('form');
                      setShowDocumentModal(true);
                    }}
                    disabled={!selectedCollection}
                    className="flex items-center gap-2 px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors disabled:opacity-50"
                  >
                    <Plus className="w-4 h-4" />
                    New Document
                  </button>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="text-center py-12 text-gray-500">
                  <RefreshCw className="w-8 h-8 animate-spin mx-auto mb-2" />
                  Loading documents...
                </div>
              ) : filteredDocuments.length === 0 ? (
                <div className="text-center py-12 text-gray-500">
                  <FileJson className="w-12 h-12 mx-auto mb-2 opacity-50" />
                  <p>No documents found</p>
                  {selectedCollection && (
                    <button
                      onClick={() => setShowDocumentModal(true)}
                      className="mt-4 text-mantis-600 hover:text-mantis-700"
                    >
                      Create your first document
                    </button>
                  )}
                </div>
              ) : (
                <div className="space-y-3">
                  {filteredDocuments.map((doc) => (
                    <div
                      key={doc.id}
                      className="border rounded-lg p-4 hover:border-mantis-300 transition-colors"
                    >
                      <div className="flex items-start justify-between mb-2">
                        <div className="flex items-center gap-2">
                          <FileJson className="w-4 h-4 text-mantis-600" />
                          <span className="text-sm font-mono text-gray-600">{doc.id}</span>
                        </div>
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => {
                              setEditingDocument(doc);
                              setNewDocumentJson(JSON.stringify(doc.data, null, 2));
                              setShowDocumentModal(true);
                            }}
                            className="p-1 hover:bg-gray-100 rounded"
                          >
                            <Edit2 className="w-4 h-4 text-gray-600" />
                          </button>
                          <button
                            onClick={() => handleDeleteDocument(doc.id)}
                            className="p-1 hover:bg-red-50 rounded"
                          >
                            <Trash2 className="w-4 h-4 text-red-600" />
                          </button>
                        </div>
                      </div>
                      <pre className="text-xs bg-gray-50 p-3 rounded overflow-x-auto">
                        {JSON.stringify(doc.data, null, 2)}
                      </pre>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Create/Edit Document Modal */}
      {showDocumentModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 max-w-2xl w-full mx-4">
            <h3 className="text-lg font-semibold mb-4">
              {editingDocument ? 'Edit Document' : 'New Document'}
            </h3>
            <div className="flex items-center gap-2 mb-3">
              <button
                onClick={() => setModalMode('form')}
                className={`px-3 py-1 rounded ${modalMode === 'form' ? 'bg-mantis-600 text-white' : 'border'}`}
              >Form</button>
              <button
                onClick={() => setModalMode('json')}
                className={`px-3 py-1 rounded ${modalMode === 'json' ? 'bg-mantis-600 text-white' : 'border'}`}
              >JSON</button>
            </div>
            {modalMode === 'json' ? (
              <textarea
                value={newDocumentJson}
                onChange={(e) => setNewDocumentJson(e.target.value)}
                className="w-full h-96 font-mono text-sm border rounded-lg p-3"
                placeholder='{"\n  "field": "value"\n}'
              />
            ) : (
              <div>
                <div className="flex justify-between items-center mb-2">
                  <span className="text-sm text-gray-600">Build document fields</span>
                  <button onClick={addFormField} className="px-3 py-1 text-sm bg-mantis-600 text-white rounded">Add Field</button>
                </div>
                {formFields.length === 0 && (
                  <div className="text-sm text-gray-500">No fields. Click Add Field to start.</div>
                )}
                <div className="space-y-2 max-h-96 overflow-y-auto pr-1">
                  {formFields.map((f, i) => (
                    <div key={i} className="grid grid-cols-12 gap-2 items-center">
                      <input
                        className="col-span-4 px-2 py-1 border rounded"
                        placeholder="field name"
                        value={f.key}
                        onChange={e => updateFormField(i, { key: e.target.value })}
                      />
                      <select
                        className="col-span-2 px-2 py-1 border rounded"
                        value={f.type}
                        onChange={e => updateFormField(i, { type: e.target.value as FieldType })}
                      >
                        <option value="string">string</option>
                        <option value="number">number</option>
                        <option value="boolean">boolean</option>
                        <option value="date">date</option>
                        <option value="json">json</option>
                      </select>
                      <input
                        className="col-span-5 px-2 py-1 border rounded font-mono"
                        placeholder={f.type === 'json' ? '{"example":1}' : 'value'}
                        value={f.value}
                        onChange={e => updateFormField(i, { value: e.target.value })}
                      />
                      <button
                        onClick={() => removeFormField(i)}
                        className="col-span-1 px-2 py-1 text-red-600 hover:bg-red-50 rounded"
                        title="Remove"
                      >✕</button>
                    </div>
                  ))}
                </div>
              </div>
            )}
            <div className="flex justify-end gap-3 mt-4">
              <button
                onClick={() => setShowDocumentModal(false)}
                className="px-4 py-2 border rounded-lg hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={handleCreateDocument}
                className="px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700"
              >
                {editingDocument ? 'Update' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
