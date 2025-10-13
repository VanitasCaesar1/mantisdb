import { useEffect, useMemo, useState } from 'react';
import { Folder, File, Upload, Download, Trash2, Search, Grid, List, FolderPlus, Info } from 'lucide-react';
import { Card, CardContent } from '../ui';
import { apiClient } from '../../api/client';

interface StorageItem {
  name: string;
  type: 'file' | 'folder';
  size?: number;
  modified?: string;
  path: string;
}

export const StorageSection = () => {
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [searchTerm, setSearchTerm] = useState('');
  const [currentPath, setCurrentPath] = useState('/');
  const [items, setItems] = useState<StorageItem[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const load = async () => {
      try {
        setLoading(true);
        const resp = await apiClient.listStorage(currentPath);
        if (resp.success) {
          setItems(((resp.data as any)?.files || []) as StorageItem[]);
        }
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [currentPath]);

  const filteredItems = useMemo(() => (
    items.filter(item => item.name.toLowerCase().includes(searchTerm.toLowerCase()))
  ), [items, searchTerm]);

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files) return;
    
    // Handle file upload to storage API
    console.log('Uploading files:', files);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Storage</h1>
          <p className="text-gray-600 mt-1">Browse database data directory and backups</p>
        </div>
        <div className="flex gap-2">
          <button className="flex items-center gap-2 px-4 py-2 border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors">
            <FolderPlus className="w-4 h-4" />
            New Folder
          </button>
          <label className="flex items-center gap-2 px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors cursor-pointer">
            <Upload className="w-4 h-4" />
            Upload Files
            <input
              type="file"
              multiple
              onChange={handleFileUpload}
              className="hidden"
            />
          </label>
        </div>
      </div>

      {/* Info Banner */}
      <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 flex items-start gap-3">
        <Info className="w-5 h-5 text-blue-600 flex-shrink-0 mt-0.5" />
        <div>
          <h3 className="text-sm font-medium text-blue-900 mb-1">Data directory explorer</h3>
          <p className="text-sm text-blue-700">Lists files under the configured data directory. Download snapshots and WALs, or navigate to backups.</p>
        </div>
      </div>

      {/* Storage Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-mantis-100 rounded-lg">
                <Folder className="w-5 h-5 text-mantis-600" />
              </div>
              <div>
                <p className="text-sm text-gray-600">Total Files</p>
                <p className="text-xl font-bold text-gray-900">{items.filter(i => i.type === 'file').length}</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-blue-100 rounded-lg">
                <File className="w-5 h-5 text-blue-600" />
              </div>
              <div>
                <p className="text-sm text-gray-600">Storage Used</p>
                <p className="text-xl font-bold text-gray-900">{
                  (() => {
                    const total = items.reduce((sum, i) => sum + (i.size || 0), 0);
                    const k = 1024; const sizes = ['B','KB','MB','GB']; const idx = Math.floor(Math.log(Math.max(total,1)) / Math.log(k));
                    return `${(total / Math.pow(k, idx)).toFixed(2)} ${sizes[idx]}`;
                  })()
                }</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-green-100 rounded-lg">
                <Upload className="w-5 h-5 text-green-600" />
              </div>
              <div>
                <p className="text-sm text-gray-600">Uploads Today</p>
                <p className="text-xl font-bold text-gray-900">0</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-yellow-100 rounded-lg">
                <Download className="w-5 h-5 text-yellow-600" />
              </div>
              <div>
                <p className="text-sm text-gray-600">Downloads Today</p>
                <p className="text-xl font-bold text-gray-900">0</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search files and folders..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-500"
          />
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setViewMode('grid')}
            className={`p-2 rounded-lg transition-colors ${
              viewMode === 'grid'
                ? 'bg-mantis-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            <Grid className="w-4 h-4" />
          </button>
          <button
            onClick={() => setViewMode('list')}
            className={`p-2 rounded-lg transition-colors ${
              viewMode === 'list'
                ? 'bg-mantis-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            <List className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm">
        <button
          onClick={() => setCurrentPath('/')}
          className="text-mantis-600 hover:text-mantis-700 font-medium"
        >
          Root
        </button>
        {currentPath.split('/').filter(Boolean).map((segment, idx, arr) => (
          <div key={idx} className="flex items-center gap-2">
            <span className="text-gray-400">/</span>
            <button
              onClick={() => setCurrentPath('/' + arr.slice(0, idx + 1).join('/'))}
              className={idx === arr.length - 1 ? 'text-gray-900 font-medium' : 'text-mantis-600 hover:text-mantis-700'}
            >
              {segment}
            </button>
          </div>
        ))}
      </div>

      {/* Files Display */}
      {loading ? (
        <Card>
          <CardContent className="p-12">
            <div className="text-center">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-mantis-600 mx-auto"></div>
              <p className="text-gray-600 mt-2">Loading files...</p>
            </div>
          </CardContent>
        </Card>
      ) : filteredItems.length === 0 ? (
        <Card>
          <CardContent className="p-12">
            <div className="text-center">
              <Folder className="w-16 h-16 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-medium text-gray-900 mb-2">No files yet</h3>
              <p className="text-gray-600 mb-4">
                Upload files to get started with storage
              </p>
              <label className="inline-flex items-center gap-2 px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors cursor-pointer">
                <Upload className="w-4 h-4" />
                Upload Files
                <input
                  type="file"
                  multiple
                  onChange={handleFileUpload}
                  className="hidden"
                />
              </label>
            </div>
          </CardContent>
        </Card>
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
          {filteredItems.map((item, idx) => (
            <Card
              key={idx}
              className="cursor-pointer hover:shadow-lg transition-all"
              onClick={() => {
                if (item.type === 'folder') {
                  setCurrentPath(item.path);
                }
              }}
            >
              <CardContent className="p-4">
                <div className="text-center">
                  {item.type === 'folder' ? (
                    <Folder className="w-12 h-12 text-mantis-600 mx-auto mb-2" />
                  ) : (
                    <File className="w-12 h-12 text-blue-600 mx-auto mb-2" />
                  )}
                  <p className="text-sm font-medium text-gray-900 truncate">{item.name}</p>
                  {item.size && (
                    <p className="text-xs text-gray-500 mt-1">{formatFileSize(item.size)}</p>
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
                    Name
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Size
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Modified
                  </th>
                  <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {filteredItems.map((item, idx) => (
                  <tr key={idx} className="hover:bg-gray-50">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center gap-2">
                        {item.type === 'folder' ? (
                          <Folder className="w-4 h-4 text-mantis-600" />
                        ) : (
                          <File className="w-4 h-4 text-blue-600" />
                        )}
                        <span className="font-medium text-gray-900">{item.name}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {item.size ? formatFileSize(item.size) : '-'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {item.modified || '-'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <div className="flex items-center justify-end gap-2">
                        {item.type === 'file' && (
                          <button className="text-blue-600 hover:text-blue-900" onClick={() => {
                            const url = apiClient.getStorageDownloadUrl(item.path);
                            window.open(url, '_blank');
                          }}>
                            <Download className="w-4 h-4" />
                          </button>
                        )}
                        <button className="text-red-600 hover:text-red-900">
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      )}
    </div>
  );
};
