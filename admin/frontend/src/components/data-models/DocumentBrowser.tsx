import React, { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';

interface Document {
  id: string;
  collection: string;
  data: any;
  created_at: number;
  updated_at: number;
  version: number;
}

export const DocumentBrowser: React.FC = () => {
  const [collections, setCollections] = useState<Array<{name: string; document_count: number}>>([]);
  const [selectedCollection, setSelectedCollection] = useState<string | null>(null);
  const [documents, setDocuments] = useState<Document[]>([]);
  const [selectedDoc, setSelectedDoc] = useState<Document | null>(null);
  const [loading, setLoading] = useState(false);
  const [showAddModal, setShowAddModal] = useState(false);
  const [showQueryModal, setShowQueryModal] = useState(false);
  const [newDocData, setNewDocData] = useState('{\n  \n}');
  const [queryFilter, setQueryFilter] = useState('{\n  \n}');

  useEffect(() => {
    loadCollections();
  }, []);

  useEffect(() => {
    if (selectedCollection) {
      loadDocuments(selectedCollection);
    }
  }, [selectedCollection]);

  const loadCollections = async () => {
    try {
      const response = await fetch('http://localhost:8081/api/documents/collections');
      const data = await response.json();
      
      if (data.collections) {
        setCollections(data.collections);
      }
    } catch (error) {
      console.error('Failed to load collections:', error);
    }
  };

  const loadDocuments = async (collection: string, filter?: any) => {
    setLoading(true);
    try {
      const body: any = { limit: 100 };
      if (filter) body.filter = filter;

      const response = await fetch(`http://localhost:8081/api/documents/${collection}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      
      const data = await response.json();
      
      if (data.success && data.documents) {
        setDocuments(data.documents);
      }
    } catch (error) {
      console.error('Failed to load documents:', error);
    } finally {
      setLoading(false);
    }
  };

  const addDocument = async () => {
    if (!selectedCollection) return;

    try {
      const data = JSON.parse(newDocData);
      
      const response = await fetch(`http://localhost:8081/api/documents/${selectedCollection}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ data }),
      });

      const result = await response.json();
      
      if (result.success) {
        setShowAddModal(false);
        setNewDocData('{\n  \n}');
        loadDocuments(selectedCollection);
        alert('Document added successfully!');
      }
    } catch (error) {
      console.error('Failed to add document:', error);
      alert('Failed to add document: ' + (error as Error).message);
    }
  };

  const deleteDocument = async (doc: Document) => {
    if (!confirm(`Delete document ${doc.id}?`)) return;

    try {
      const response = await fetch(
        `http://localhost:8081/api/documents/${doc.collection}/${doc.id}`,
        { method: 'DELETE' }
      );

      const data = await response.json();
      
      if (data.success) {
        setSelectedDoc(null);
        loadDocuments(doc.collection);
        alert('Document deleted successfully!');
      }
    } catch (error) {
      console.error('Failed to delete document:', error);
      alert('Failed to delete document');
    }
  };

  const executeQuery = async () => {
    if (!selectedCollection) return;

    try {
      const filter = JSON.parse(queryFilter);
      loadDocuments(selectedCollection, filter);
      setShowQueryModal(false);
    } catch (error) {
      alert('Invalid JSON filter: ' + (error as Error).message);
    }
  };

  const aggregateExample = async () => {
    if (!selectedCollection) return;

    try {
      const pipeline = [
        { $match: {} },
        { $limit: 10 }
      ];

      const response = await fetch(
        `http://localhost:8081/api/documents/${selectedCollection}/aggregate`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ pipeline }),
        }
      );

      const data = await response.json();
      
      if (data.success) {
        setDocuments(data.results);
      }
    } catch (error) {
      console.error('Failed to aggregate:', error);
    }
  };

  return (
    <div className="space-y-4">
      {/* Collections Bar */}
      <Card>
        <CardHeader>
          <div className="flex justify-between items-center">
            <CardTitle>Collections</CardTitle>
            <div className="flex gap-2">
              <Button variant="secondary" onClick={() => setShowQueryModal(true)} disabled={!selectedCollection}>
                Query
              </Button>
              <Button variant="secondary" onClick={aggregateExample} disabled={!selectedCollection}>
                Aggregate
              </Button>
              <Button onClick={() => setShowAddModal(true)} disabled={!selectedCollection}>
                Add Document
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            {collections.map(coll => (
              <button
                key={coll.name}
                onClick={() => setSelectedCollection(coll.name)}
                className={`px-4 py-2 rounded ${
                  selectedCollection === coll.name
                    ? 'bg-mantis-600 text-white'
                    : 'bg-gray-100 hover:bg-gray-200'
                }`}
              >
                {coll.name} ({coll.document_count})
              </button>
            ))}
            {collections.length === 0 && (
              <div className="text-gray-500">No collections found</div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Documents Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Documents List */}
        <Card>
          <CardHeader>
            <CardTitle>
              Documents {selectedCollection && `in ${selectedCollection}`}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="text-center py-4">Loading...</div>
            ) : documents.length === 0 ? (
              <div className="text-center py-4 text-gray-500">No documents found</div>
            ) : (
              <div className="space-y-2 max-h-96 overflow-y-auto">
                {documents.map(doc => (
                  <div
                    key={doc.id}
                    onClick={() => setSelectedDoc(doc)}
                    className={`p-3 rounded cursor-pointer border hover:bg-gray-50 ${
                      selectedDoc?.id === doc.id ? 'border-mantis-600 bg-mantis-50' : ''
                    }`}
                  >
                    <div className="font-mono text-sm font-semibold">{doc.id}</div>
                    <div className="text-xs text-gray-500 mt-1">
                      Version {doc.version} • Updated {new Date(doc.updated_at * 1000).toLocaleString()}
                    </div>
                    <pre className="text-xs text-gray-600 mt-2 truncate">
                      {JSON.stringify(doc.data)}
                    </pre>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Document Details */}
        <Card>
          <CardHeader>
            <div className="flex justify-between items-center">
              <CardTitle>
                {selectedDoc ? `Document: ${selectedDoc.id}` : 'Select a document'}
              </CardTitle>
              {selectedDoc && (
                <Button variant="secondary" onClick={() => deleteDocument(selectedDoc)}>
                  Delete
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent>
            {selectedDoc ? (
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Data</label>
                  <pre className="p-4 bg-gray-50 rounded border overflow-x-auto max-h-96">
                    {JSON.stringify(selectedDoc.data, null, 2)}
                  </pre>
                </div>
                
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <span className="font-medium">Collection:</span> {selectedDoc.collection}
                  </div>
                  <div>
                    <span className="font-medium">Version:</span> {selectedDoc.version}
                  </div>
                  <div>
                    <span className="font-medium">Created:</span> {new Date(selectedDoc.created_at * 1000).toLocaleString()}
                  </div>
                  <div>
                    <span className="font-medium">Updated:</span> {new Date(selectedDoc.updated_at * 1000).toLocaleString()}
                  </div>
                </div>
              </div>
            ) : (
              <div className="text-center py-12 text-gray-500">
                Select a document to view details
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Add Document Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card className="w-full max-w-2xl">
            <CardHeader>
              <CardTitle>Add Document to {selectedCollection}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Document Data (JSON)</label>
                  <textarea
                    value={newDocData}
                    onChange={(e) => setNewDocData(e.target.value)}
                    className="w-full px-3 py-2 border rounded font-mono"
                    rows={12}
                  />
                </div>
                
                <div className="flex gap-2">
                  <Button onClick={addDocument}>
                    Add Document
                  </Button>
                  <Button variant="secondary" onClick={() => setShowAddModal(false)}>
                    Cancel
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Query Modal */}
      {showQueryModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card className="w-full max-w-2xl">
            <CardHeader>
              <CardTitle>Query {selectedCollection}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Filter (MongoDB-style)</label>
                  <textarea
                    value={queryFilter}
                    onChange={(e) => setQueryFilter(e.target.value)}
                    className="w-full px-3 py-2 border rounded font-mono"
                    rows={8}
                    placeholder='{"field": "value", "age": {"$gt": 18}}'
                  />
                  <div className="text-xs text-gray-500 mt-2">
                    Supported operators: $eq, $ne, $gt, $gte, $lt, $lte
                  </div>
                </div>
                
                <div className="flex gap-2">
                  <Button onClick={executeQuery}>
                    Execute Query
                  </Button>
                  <Button variant="secondary" onClick={() => setShowQueryModal(false)}>
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
