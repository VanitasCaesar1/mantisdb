import React, { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';

interface FileItem {
  name: string;
  type: 'file' | 'folder';
  size?: number;
  modified?: Date;
}

export const StorageManager: React.FC = () => {
  const [currentPath] = useState('/');
  const [files, setFiles] = useState<FileItem[]>([
    { name: 'uploads', type: 'folder', modified: new Date() },
    { name: 'images', type: 'folder', modified: new Date() },
    { name: 'documents', type: 'folder', modified: new Date() },
    { name: 'example.txt', type: 'file', size: 1024, modified: new Date() },
  ]);
  const [selectedFiles, setSelectedFiles] = useState<Set<string>>(new Set());
  const [uploading, setUploading] = useState(false);

  const handleFileUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const uploadedFiles = event.target.files;
    if (!uploadedFiles || uploadedFiles.length === 0) return;

    setUploading(true);
    try {
      // Mock upload - in production, upload to storage API
      await new Promise(resolve => setTimeout(resolve, 1000));
      
      const newFiles: FileItem[] = Array.from(uploadedFiles).map(file => ({
        name: file.name,
        type: 'file' as const,
        size: file.size,
        modified: new Date(),
      }));

      setFiles(prev => [...prev, ...newFiles]);
    } catch (error) {
      console.error('Upload error:', error);
    } finally {
      setUploading(false);
    }
  };

  const handleFileSelect = (fileName: string) => {
    const newSelection = new Set(selectedFiles);
    if (newSelection.has(fileName)) {
      newSelection.delete(fileName);
    } else {
      newSelection.add(fileName);
    }
    setSelectedFiles(newSelection);
  };

  const handleDelete = () => {
    if (selectedFiles.size === 0) return;
    if (!confirm(`Delete ${selectedFiles.size} item(s)?`)) return;

    setFiles(prev => prev.filter(f => !selectedFiles.has(f.name)));
    setSelectedFiles(new Set());
  };

  const formatSize = (bytes?: number) => {
    if (!bytes) return '-';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const formatDate = (date?: Date) => {
    if (!date) return '-';
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <div className="flex justify-between items-center">
            <div>
              <CardTitle>Storage Manager</CardTitle>
              <p className="text-sm text-gray-600 mt-1">
                Current path: {currentPath}
              </p>
            </div>
            <div className="flex gap-2">
              <label className="cursor-pointer">
                <input
                  type="file"
                  multiple
                  onChange={handleFileUpload}
                  className="hidden"
                  disabled={uploading}
                />
                <Button disabled={uploading}>
                  {uploading ? 'Uploading...' : '📁 Upload Files'}
                </Button>
              </label>
              {selectedFiles.size > 0 && (
                <Button variant="danger" onClick={handleDelete}>
                  🗑️ Delete ({selectedFiles.size})
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-3 py-3 text-left">
                    <input
                      type="checkbox"
                      onChange={(e) => {
                        if (e.target.checked) {
                          setSelectedFiles(new Set(files.map(f => f.name)));
                        } else {
                          setSelectedFiles(new Set());
                        }
                      }}
                    />
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Name
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Type
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Size
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Modified
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {files.map((file) => (
                  <tr
                    key={file.name}
                    className={selectedFiles.has(file.name) ? 'bg-blue-50' : 'hover:bg-gray-50'}
                  >
                    <td className="px-3 py-4">
                      <input
                        type="checkbox"
                        checked={selectedFiles.has(file.name)}
                        onChange={() => handleFileSelect(file.name)}
                      />
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <span className="mr-2">
                          {file.type === 'folder' ? '📁' : '📄'}
                        </span>
                        <span className="text-sm font-medium text-gray-900">
                          {file.name}
                        </span>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                      {file.type}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                      {formatSize(file.size)}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                      {formatDate(file.modified)}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      <div className="flex gap-2">
                        <button className="text-blue-600 hover:text-blue-800">
                          Download
                        </button>
                        <button className="text-red-600 hover:text-red-800">
                          Delete
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {/* Storage Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardContent className="p-6">
            <div className="text-sm text-gray-600">Total Files</div>
            <div className="text-2xl font-bold text-gray-900 mt-1">
              {files.filter(f => f.type === 'file').length}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-6">
            <div className="text-sm text-gray-600">Total Folders</div>
            <div className="text-2xl font-bold text-gray-900 mt-1">
              {files.filter(f => f.type === 'folder').length}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-6">
            <div className="text-sm text-gray-600">Storage Used</div>
            <div className="text-2xl font-bold text-gray-900 mt-1">
              {formatSize(files.reduce((sum, f) => sum + (f.size || 0), 0))}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
};
