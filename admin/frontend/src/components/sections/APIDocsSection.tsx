import { useState, useEffect } from 'react';
import { Code, Copy, Check, Book, Database, FileJson, Table } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '../ui';
import { getAdminPort } from '../../config/api';

interface Endpoint {
  method: string;
  path: string;
  description: string;
  category: string;
  example?: string;
  requestBody?: any;
  response?: any;
}

export const APIDocsSection = () => {
  const [selectedEndpoint, setSelectedEndpoint] = useState<Endpoint | null>(null);
  const [copiedCode, setCopiedCode] = useState<string | null>(null);
  const [activeCategory, setActiveCategory] = useState<string>('all');
  const [baseUrl, setBaseUrl] = useState<string>('');

  // Dynamically detect the admin server port
  useEffect(() => {
    getAdminPort().then(port => {
      setBaseUrl(`${window.location.protocol}//${window.location.hostname}:${port}/api`);
    });
  }, []);

  const endpoints: Endpoint[] = [
    // Key-Value Endpoints
    {
      method: 'GET',
      path: '/kv/{key}',
      description: 'Get a value by key',
      category: 'Key-Value',
      example: `curl -X GET ${baseUrl}/kv/mykey \\
  -H 'Authorization: Bearer YOUR_TOKEN'`,
      response: { key: 'mykey', value: 'myvalue' }
    },
    {
      method: 'POST',
      path: '/kv/{key}',
      description: 'Set a key-value pair',
      category: 'Key-Value',
      requestBody: { value: 'myvalue', ttl: 3600 },
      example: `curl -X POST ${baseUrl}/kv/mykey \\
  -H 'Authorization: Bearer YOUR_TOKEN' \\
  -H 'Content-Type: application/json' \\
  -d '{"value":"myvalue","ttl":3600}'`,
      response: { key: 'mykey', success: true }
    },
    {
      method: 'DELETE',
      path: '/kv/{key}',
      description: 'Delete a key',
      category: 'Key-Value',
      example: `curl -X DELETE ${baseUrl}/kv/mykey \\
  -H 'Authorization: Bearer YOUR_TOKEN'`,
      response: { key: 'mykey', deleted: true }
    },
    {
      method: 'POST',
      path: '/kv/batch',
      description: 'Execute batch operations',
      category: 'Key-Value',
      requestBody: {
        operations: [
          { type: 'set', key: 'key1', value: 'value1' },
          { type: 'get', key: 'key2' },
          { type: 'delete', key: 'key3' }
        ],
        atomic: false
      },
      example: `curl -X POST ${baseUrl}/kv/batch \\
  -H 'Authorization: Bearer YOUR_TOKEN' \\
  -H 'Content-Type: application/json' \\
  -d '{"operations":[{"type":"set","key":"key1","value":"value1"}]}'`,
      response: { results: [], success: true }
    },
    // Document Endpoints
    {
      method: 'GET',
      path: '/docs/{collection}/{id}',
      description: 'Get a document by ID',
      category: 'Documents',
      example: `curl -X GET ${baseUrl}/docs/users/user123 \\
  -H 'Authorization: Bearer YOUR_TOKEN'`,
      response: { id: 'user123', collection: 'users', data: {} }
    },
    {
      method: 'POST',
      path: '/docs/{collection}',
      description: 'Create a new document',
      category: 'Documents',
      requestBody: { id: 'user123', data: { name: 'John', email: 'john@example.com' } },
      example: `curl -X POST ${baseUrl}/docs/users \\
  -H 'Authorization: Bearer YOUR_TOKEN' \\
  -H 'Content-Type: application/json' \\
  -d '{"id":"user123","data":{"name":"John"}}'`,
      response: { id: 'user123', collection: 'users', data: {} }
    },
    {
      method: 'PUT',
      path: '/docs/{collection}/{id}',
      description: 'Update a document',
      category: 'Documents',
      requestBody: { data: { name: 'John Updated' } },
      example: `curl -X PUT ${baseUrl}/docs/users/user123 \\
  -H 'Authorization: Bearer YOUR_TOKEN' \\
  -H 'Content-Type: application/json' \\
  -d '{"data":{"name":"John Updated"}}'`,
      response: { id: 'user123', collection: 'users', data: {} }
    },
    {
      method: 'DELETE',
      path: '/docs/{collection}/{id}',
      description: 'Delete a document',
      category: 'Documents',
      example: `curl -X DELETE ${baseUrl}/docs/users/user123 \\
  -H 'Authorization: Bearer YOUR_TOKEN'`,
      response: { collection: 'users', id: 'user123', deleted: true }
    },
    {
      method: 'POST',
      path: '/docs/query',
      description: 'Query documents',
      category: 'Documents',
      requestBody: { collection: 'users', filter: { age: { $gt: 18 } }, limit: 10 },
      example: `curl -X POST ${baseUrl}/docs/query \\
  -H 'Authorization: Bearer YOUR_TOKEN' \\
  -H 'Content-Type: application/json' \\
  -d '{"collection":"users","filter":{"age":{"$gt":18}}}'`,
      response: { documents: [], count: 0 }
    },
    // Columnar/Table Endpoints
    {
      method: 'GET',
      path: '/tables/{name}',
      description: 'Get table schema',
      category: 'Tables',
      example: `curl -X GET ${baseUrl}/tables/users \\
  -H 'Authorization: Bearer YOUR_TOKEN'`,
      response: { name: 'users', columns: [] }
    },
    {
      method: 'POST',
      path: '/tables/{name}',
      description: 'Create a new table',
      category: 'Tables',
      requestBody: { columns: [{ name: 'id', type: 'string' }, { name: 'name', type: 'string' }] },
      example: `curl -X POST ${baseUrl}/tables/users \\
  -H 'Authorization: Bearer YOUR_TOKEN' \\
  -H 'Content-Type: application/json' \\
  -d '{"columns":[{"name":"id","type":"string"}]}'`,
      response: { name: 'users', columns: [] }
    },
    {
      method: 'POST',
      path: '/tables/{name}/insert',
      description: 'Insert rows into table',
      category: 'Tables',
      requestBody: { rows: [{ id: '1', name: 'John' }] },
      example: `curl -X POST ${baseUrl}/tables/users/insert \\
  -H 'Authorization: Bearer YOUR_TOKEN' \\
  -H 'Content-Type: application/json' \\
  -d '{"rows":[{"id":"1","name":"John"}]}'`,
      response: { table: 'users', rows_inserted: 1, success: true }
    },
    {
      method: 'POST',
      path: '/tables/query',
      description: 'Query table data',
      category: 'Tables',
      requestBody: { table: 'users', limit: 10, offset: 0 },
      example: `curl -X POST ${baseUrl}/tables/query \\
  -H 'Authorization: Bearer YOUR_TOKEN' \\
  -H 'Content-Type: application/json' \\
  -d '{"table":"users","limit":10}'`,
      response: { rows: [], count: 0 }
    },
    // System Endpoints
    {
      method: 'GET',
      path: '/stats',
      description: 'Get database statistics',
      category: 'System',
      example: `curl -X GET ${baseUrl}/stats \\
  -H 'Authorization: Bearer YOUR_TOKEN'`,
      response: { active_connections: 0, total_records: 0 }
    },
    {
      method: 'GET',
      path: '/version',
      description: 'Get API version info',
      category: 'System',
      example: `curl -X GET ${baseUrl}/version`,
      response: { version: '1.0.0', build: 'dev' }
    },
    {
      method: 'GET',
      path: '/health',
      description: 'Health check endpoint',
      category: 'System',
      example: `curl -X GET ${baseUrl}/health`,
      response: { status: 'healthy', timestamp: Date.now() }
    }
  ];

  const categories = ['all', ...Array.from(new Set(endpoints.map(e => e.category)))];

  const filteredEndpoints = activeCategory === 'all'
    ? endpoints
    : endpoints.filter(e => e.category === activeCategory);

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedCode(id);
    setTimeout(() => setCopiedCode(null), 2000);
  };

  const getMethodColor = (method: string) => {
    const colors: Record<string, string> = {
      GET: 'bg-blue-100 text-blue-800',
      POST: 'bg-green-100 text-green-800',
      PUT: 'bg-yellow-100 text-yellow-800',
      DELETE: 'bg-red-100 text-red-800',
      PATCH: 'bg-purple-100 text-purple-800'
    };
    return colors[method] || 'bg-gray-100 text-gray-800';
  };

  const getCategoryIcon = (category: string) => {
    const icons: Record<string, any> = {
      'Key-Value': Database,
      'Documents': FileJson,
      'Tables': Table,
      'System': Code
    };
    const Icon = icons[category] || Code;
    return <Icon className="w-4 h-4" />;
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">API Documentation</h1>
        <p className="text-gray-600 mt-1">Explore and test MantisDB REST API endpoints</p>
      </div>

      {/* API Info */}
      <Card>
        <CardHeader>
          <CardTitle>API Information</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-600">Base URL</span>
              <div className="flex items-center gap-2">
                <code className="text-sm font-mono bg-gray-100 px-2 py-1 rounded">{baseUrl}</code>
                <button
                  onClick={() => copyToClipboard(baseUrl, 'base-url')}
                  className="p-1 hover:bg-gray-100 rounded transition-colors"
                >
                  {copiedCode === 'base-url' ? (
                    <Check className="w-4 h-4 text-green-600" />
                  ) : (
                    <Copy className="w-4 h-4 text-gray-400" />
                  )}
                </button>
              </div>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-600">Authentication</span>
              <span className="text-sm font-medium">Bearer Token</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-600">Content-Type</span>
              <span className="text-sm font-mono">application/json</span>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Category Filter */}
      <div className="flex gap-2 overflow-x-auto pb-2">
        {categories.map((category) => (
          <button
            key={category}
            onClick={() => setActiveCategory(category)}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg whitespace-nowrap transition-colors ${
              activeCategory === category
                ? 'bg-mantis-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            {category !== 'all' && getCategoryIcon(category)}
            {category.charAt(0).toUpperCase() + category.slice(1)}
          </button>
        ))}
      </div>

      {/* Endpoints List */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Endpoints */}
        <div className="space-y-3">
          <h2 className="text-lg font-semibold text-gray-900">Endpoints</h2>
          <div className="space-y-2">
            {filteredEndpoints.map((endpoint, idx) => (
              <Card
                key={idx}
                className={`cursor-pointer hover:shadow-md transition-all ${
                  selectedEndpoint === endpoint ? 'ring-2 ring-mantis-500' : ''
                }`}
                onClick={() => setSelectedEndpoint(endpoint)}
              >
                <CardContent className="p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-1">
                        <span className={`px-2 py-0.5 text-xs font-medium rounded ${getMethodColor(endpoint.method)}`}>
                          {endpoint.method}
                        </span>
                        <code className="text-sm font-mono text-gray-900">{endpoint.path}</code>
                      </div>
                      <p className="text-sm text-gray-600">{endpoint.description}</p>
                    </div>
                    {getCategoryIcon(endpoint.category)}
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>

        {/* Endpoint Details */}
        <div className="lg:sticky lg:top-6 space-y-4">
          {selectedEndpoint ? (
            <>
              <Card>
                <CardHeader>
                  <div className="flex items-center gap-2">
                    <span className={`px-2 py-1 text-xs font-medium rounded ${getMethodColor(selectedEndpoint.method)}`}>
                      {selectedEndpoint.method}
                    </span>
                    <CardTitle className="text-base font-mono">{selectedEndpoint.path}</CardTitle>
                  </div>
                </CardHeader>
                <CardContent>
                  <p className="text-sm text-gray-600">{selectedEndpoint.description}</p>
                </CardContent>
              </Card>

              {selectedEndpoint.requestBody && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">Request Body</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="relative">
                      <pre className="bg-gray-900 text-gray-100 p-4 rounded-lg text-sm overflow-x-auto">
                        {JSON.stringify(selectedEndpoint.requestBody, null, 2)}
                      </pre>
                      <button
                        onClick={() => copyToClipboard(JSON.stringify(selectedEndpoint.requestBody, null, 2), 'request')}
                        className="absolute top-2 right-2 p-2 bg-gray-800 hover:bg-gray-700 rounded transition-colors"
                      >
                        {copiedCode === 'request' ? (
                          <Check className="w-4 h-4 text-green-400" />
                        ) : (
                          <Copy className="w-4 h-4 text-gray-400" />
                        )}
                      </button>
                    </div>
                  </CardContent>
                </Card>
              )}

              {selectedEndpoint.example && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">Example Request</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="relative">
                      <pre className="bg-gray-900 text-gray-100 p-4 rounded-lg text-sm overflow-x-auto">
                        {selectedEndpoint.example}
                      </pre>
                      <button
                        onClick={() => copyToClipboard(selectedEndpoint.example!, 'example')}
                        className="absolute top-2 right-2 p-2 bg-gray-800 hover:bg-gray-700 rounded transition-colors"
                      >
                        {copiedCode === 'example' ? (
                          <Check className="w-4 h-4 text-green-400" />
                        ) : (
                          <Copy className="w-4 h-4 text-gray-400" />
                        )}
                      </button>
                    </div>
                  </CardContent>
                </Card>
              )}

              {selectedEndpoint.response && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">Example Response</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="relative">
                      <pre className="bg-gray-900 text-gray-100 p-4 rounded-lg text-sm overflow-x-auto">
                        {JSON.stringify(selectedEndpoint.response, null, 2)}
                      </pre>
                      <button
                        onClick={() => copyToClipboard(JSON.stringify(selectedEndpoint.response, null, 2), 'response')}
                        className="absolute top-2 right-2 p-2 bg-gray-800 hover:bg-gray-700 rounded transition-colors"
                      >
                        {copiedCode === 'response' ? (
                          <Check className="w-4 h-4 text-green-400" />
                        ) : (
                          <Copy className="w-4 h-4 text-gray-400" />
                        )}
                      </button>
                    </div>
                  </CardContent>
                </Card>
              )}
            </>
          ) : (
            <Card>
              <CardContent className="p-12">
                <div className="text-center">
                  <Book className="w-16 h-16 text-gray-400 mx-auto mb-4" />
                  <h3 className="text-lg font-medium text-gray-900 mb-2">Select an endpoint</h3>
                  <p className="text-gray-600">
                    Click on an endpoint to view details, examples, and try it out
                  </p>
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
};
