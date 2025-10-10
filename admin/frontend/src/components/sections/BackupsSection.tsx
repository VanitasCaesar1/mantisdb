import { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Input } from '../ui';
import { BackupIcon } from '../icons';
import { apiClient } from '../../api/client';

interface Backup {
  id: string;
  status: string;
  created_at: string;
  completed_at?: string;
  size_bytes?: number;
  record_count?: number;
  checksum?: string;
  tags?: { [key: string]: string };
  error?: string;
  progress_percent?: number;
}

export function BackupsSection() {
  const [backups, setBackups] = useState<Backup[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [backupName, setBackupName] = useState('');

  useEffect(() => {
    fetchBackups();
  }, []);

  const fetchBackups = async () => {
    try {
      setLoading(true);
      const resp = await apiClient.getBackups();
      if (resp.success) {
        const list = (resp.data as any)?.backups || [];
        setBackups(list);
      }
    } catch (err) {
      console.error('Failed to fetch backups:', err);
    } finally {
      setLoading(false);
    }
  };

  const createBackup = async () => {
    try {
      setCreating(true);
      const resp = await apiClient.createBackup({
        tags: backupName.trim() ? { name: backupName.trim() } : {},
        description: backupName.trim() || 'Manual backup',
      });
      if (resp.success) {
        setBackupName('');
        const backupId = (resp.data as any)?.backup_id;
        const pollInterval = setInterval(async () => {
          const status = await apiClient.getBackupStatus(backupId);
          if (status.success) {
            const st = (status.data as any)?.backup;
            if (st?.status === 'completed' || st?.status === 'failed') {
              clearInterval(pollInterval);
              fetchBackups();
              setCreating(false);
            }
          }
        }, 500);
        setTimeout(() => {
          clearInterval(pollInterval);
          fetchBackups();
          setCreating(false);
        }, 30000);
      }
    } catch (err) {
      console.error('Failed to create backup:', err);
      setCreating(false);
    }
  };

  const restoreBackup = async (backupId: string) => {
    if (!confirm('Are you sure you want to restore this backup? This will overwrite current data.')) {
      return;
    }

    try {
      const resp = await apiClient.restoreBackup(backupId, {});
      if (resp.success) alert('Backup restored successfully');
    } catch (err) {
      console.error('Failed to restore backup:', err);
      alert('Failed to restore backup');
    }
  };

  const deleteBackup = async (backupId: string) => {
    if (!confirm('Are you sure you want to delete this backup?')) {
      return;
    }

    try {
      const resp = await apiClient.deleteBackup(backupId);
      if (resp.success) fetchBackups();
    } catch (err) {
      console.error('Failed to delete backup:', err);
    }
  };

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div className="space-y-6">
      {/* Create Backup */}
      <Card>
        <CardHeader>
          <CardTitle>Create New Backup</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex space-x-4">
            <div className="flex-1">
              <Input
                type="text"
                placeholder="Backup name (optional)"
                value={backupName}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setBackupName(e.target.value)}
              />
            </div>
            <Button onClick={createBackup} disabled={creating}>
              {creating ? 'Creating...' : 'Create Backup'}
            </Button>
          </div>
          <p className="mt-2 text-sm text-gray-600">
            Create a full backup of the database. If no name is provided, a timestamp will be used.
          </p>
        </CardContent>
      </Card>

      {/* Backups List */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Available Backups</CardTitle>
            <Button variant="secondary" onClick={fetchBackups} disabled={loading}>
              {loading ? 'Loading...' : 'Refresh'}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="text-center py-12">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-mantis-600 mx-auto"></div>
              <p className="text-gray-600 mt-2">Loading backups...</p>
            </div>
          ) : backups.length === 0 ? (
            <div className="text-center py-12">
              <BackupIcon className="w-12 h-12 mx-auto text-gray-400 mb-4" />
              <h3 className="text-lg font-medium text-gray-900 mb-2">No Backups Found</h3>
              <p className="text-gray-600">Create your first backup to get started.</p>
            </div>
          ) : (
            <div className="space-y-3">
              {backups.map((backup) => (
                <div
                  key={backup.id}
                  className="p-4 border border-gray-200 rounded-lg hover:border-gray-300 transition-colors"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex-1">
                      <div className="flex items-center space-x-3">
                        <h4 className="font-medium text-gray-900">
                          {backup.tags?.name || backup.tags?.description || backup.id}
                        </h4>
                        <span className={`px-2 py-1 text-xs rounded-full ${
                          backup.status === 'completed' ? 'bg-green-100 text-green-800' :
                          backup.status === 'creating' ? 'bg-yellow-100 text-yellow-800' :
                          'bg-red-100 text-red-800'
                        }`}>
                          {backup.status} {backup.status === 'creating' ? `${backup.progress_percent || 0}%` : ''}
                        </span>
                      </div>
                      <div className="mt-2 flex items-center space-x-4 text-sm text-gray-600">
                        <span>Size: {formatBytes(backup.size_bytes || 0)}</span>
                        <span>Created: {new Date(backup.created_at).toLocaleString()}</span>
                        {backup.record_count && <span>Records: {backup.record_count.toLocaleString()}</span>}
                      </div>
                    </div>
                    <div className="flex space-x-2 ml-4">
                      <Button
                        variant="secondary"
                        onClick={() => restoreBackup(backup.id)}
                        disabled={backup.status !== 'completed'}
                      >
                        Restore
                      </Button>
                      <Button
                        variant="secondary"
                        onClick={() => deleteBackup(backup.id)}
                      >
                        Delete
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
