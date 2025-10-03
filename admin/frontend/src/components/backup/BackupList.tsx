import React, { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Badge, Modal } from '../ui';
import { RefreshIcon, PlusIcon, BackupIcon } from '../icons';
import { formatRelativeTime, formatBytes } from '../../utils';
import type { BackupInfo } from '../../types';

export interface BackupListProps {
  backups: BackupInfo[];
  loading?: boolean;
  onRefresh: () => void;
  onCreate: () => void;
  onRestore: (backupId: string) => void;
  onDelete: (backupId: string) => void;
  onDownload?: (backupId: string) => void;
}

const BackupList: React.FC<BackupListProps> = ({
  backups,
  loading = false,
  onRefresh,
  onCreate,
  onRestore,
  onDelete,
  onDownload
}) => {
  const [selectedBackup, setSelectedBackup] = useState<BackupInfo | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [restoreConfirm, setRestoreConfirm] = useState<string | null>(null);

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed': return 'success';
      case 'running': return 'info';
      case 'pending': return 'warning';
      case 'failed': return 'danger';
      default: return 'default';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return (
          <svg className="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      case 'running':
        return (
          <svg className="w-4 h-4 text-blue-500 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        );
      case 'pending':
        return (
          <svg className="w-4 h-4 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      case 'failed':
        return (
          <svg className="w-4 h-4 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      default:
        return null;
    }
  };

  const sortedBackups = [...backups].sort((a, b) => 
    new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  );

  const completedBackups = backups.filter(b => b.status === 'completed');
  const runningBackups = backups.filter(b => b.status === 'running');
  const failedBackups = backups.filter(b => b.status === 'failed');

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Database Backups</CardTitle>
              <p className="text-sm text-gray-600 mt-1">
                {backups.length} total backups • {completedBackups.length} completed • {runningBackups.length} running
              </p>
            </div>
            <div className="flex items-center space-x-3">
              <Button
                variant="secondary"
                size="sm"
                onClick={onRefresh}
                loading={loading}
              >
                <RefreshIcon className="w-4 h-4 mr-2" />
                Refresh
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={onCreate}
              >
                <PlusIcon className="w-4 h-4 mr-2" />
                Create Backup
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {/* Summary Cards */}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
            <div className="p-4 bg-green-50 rounded-lg border border-green-200">
              <div className="flex items-center space-x-2">
                <svg className="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div>
                  <p className="text-sm font-medium text-green-800">Completed</p>
                  <p className="text-2xl font-bold text-green-900">{completedBackups.length}</p>
                </div>
              </div>
            </div>
            
            <div className="p-4 bg-blue-50 rounded-lg border border-blue-200">
              <div className="flex items-center space-x-2">
                <svg className="w-5 h-5 text-blue-500 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                <div>
                  <p className="text-sm font-medium text-blue-800">Running</p>
                  <p className="text-2xl font-bold text-blue-900">{runningBackups.length}</p>
                </div>
              </div>
            </div>
            
            <div className="p-4 bg-red-50 rounded-lg border border-red-200">
              <div className="flex items-center space-x-2">
                <svg className="w-5 h-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div>
                  <p className="text-sm font-medium text-red-800">Failed</p>
                  <p className="text-2xl font-bold text-red-900">{failedBackups.length}</p>
                </div>
              </div>
            </div>
            
            <div className="p-4 bg-gray-50 rounded-lg border border-gray-200">
              <div className="flex items-center space-x-2">
                <BackupIcon className="w-5 h-5 text-gray-500" />
                <div>
                  <p className="text-sm font-medium text-gray-800">Total Size</p>
                  <p className="text-2xl font-bold text-gray-900">
                    {formatBytes(completedBackups.reduce((sum, backup) => sum + (backup.size || 0), 0))}
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Backups List */}
          {loading && backups.length === 0 ? (
            <div className="text-center py-8">
              <div className="animate-spin w-8 h-8 border-2 border-mantis-600 border-t-transparent rounded-full mx-auto mb-4"></div>
              <p className="text-gray-600">Loading backups...</p>
            </div>
          ) : sortedBackups.length === 0 ? (
            <div className="text-center py-8">
              <div className="text-gray-400 mb-4">
                <BackupIcon className="w-12 h-12 mx-auto" />
              </div>
              <h3 className="text-lg font-medium text-gray-900 mb-2">No backups found</h3>
              <p className="text-gray-600 mb-4">
                Create your first backup to ensure data safety and recovery capabilities.
              </p>
              <Button variant="primary" onClick={onCreate}>
                <PlusIcon className="w-4 h-4 mr-2" />
                Create First Backup
              </Button>
            </div>
          ) : (
            <div className="space-y-3">
              {sortedBackups.map((backup) => (
                <div
                  key={backup.id}
                  className="border border-gray-200 rounded-lg p-4 hover:bg-gray-50 transition-colors cursor-pointer"
                  onClick={() => setSelectedBackup(backup)}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-3">
                      <div className="flex-shrink-0">
                        {getStatusIcon(backup.status)}
                      </div>
                      <div className="flex-1">
                        <div className="flex items-center space-x-2 mb-1">
                          <h4 className="text-sm font-medium text-gray-900">
                            {backup.name}
                          </h4>
                          <Badge variant={getStatusColor(backup.status)} size="sm">
                            {backup.status}
                          </Badge>
                          <Badge variant={backup.type === 'manual' ? 'info' : 'default'} size="sm">
                            {backup.type}
                          </Badge>
                        </div>
                        <div className="flex items-center space-x-4 text-sm text-gray-600">
                          <span>Created {formatRelativeTime(backup.created_at)}</span>
                          {backup.size && (
                            <span>{formatBytes(backup.size)}</span>
                          )}
                          {backup.status === 'running' && backup.progress && (
                            <span>{backup.progress}% complete</span>
                          )}
                        </div>
                        {backup.error_message && (
                          <p className="text-sm text-red-600 mt-1">
                            Error: {backup.error_message}
                          </p>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center space-x-2">
                      {backup.status === 'completed' && (
                        <>
                          {onDownload && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={(e) => {
                                e.stopPropagation();
                                onDownload(backup.id);
                              }}
                            >
                              Download
                            </Button>
                          )}
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={(e) => {
                              e.stopPropagation();
                              setRestoreConfirm(backup.id);
                            }}
                          >
                            Restore
                          </Button>
                        </>
                      )}
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation();
                          setDeleteConfirm(backup.id);
                        }}
                        className="text-red-600 hover:text-red-700"
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

      {/* Backup Details Modal */}
      {selectedBackup && (
        <Modal
          isOpen={true}
          onClose={() => setSelectedBackup(null)}
          title="Backup Details"
          size="lg"
        >
          <div className="space-y-4">
            <div className="flex items-center space-x-2">
              {getStatusIcon(selectedBackup.status)}
              <h3 className="text-lg font-semibold text-gray-900">
                {selectedBackup.name}
              </h3>
              <Badge variant={getStatusColor(selectedBackup.status)}>
                {selectedBackup.status}
              </Badge>
            </div>

            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span className="font-medium text-gray-600">Type:</span>
                <p className="text-gray-900">{selectedBackup.type}</p>
              </div>
              <div>
                <span className="font-medium text-gray-600">Created:</span>
                <p className="text-gray-900">{selectedBackup.created_at.toLocaleString()}</p>
              </div>
              {selectedBackup.completed_at && (
                <div>
                  <span className="font-medium text-gray-600">Completed:</span>
                  <p className="text-gray-900">{selectedBackup.completed_at.toLocaleString()}</p>
                </div>
              )}
              {selectedBackup.size && (
                <div>
                  <span className="font-medium text-gray-600">Size:</span>
                  <p className="text-gray-900">{formatBytes(selectedBackup.size)}</p>
                </div>
              )}
              {selectedBackup.progress && selectedBackup.status === 'running' && (
                <div>
                  <span className="font-medium text-gray-600">Progress:</span>
                  <p className="text-gray-900">{selectedBackup.progress}%</p>
                </div>
              )}
            </div>

            {selectedBackup.error_message && (
              <div className="bg-red-50 border border-red-200 rounded-lg p-4">
                <h4 className="font-medium text-red-800 mb-2">Error Details</h4>
                <p className="text-red-700 text-sm">{selectedBackup.error_message}</p>
              </div>
            )}

            <div className="flex justify-end space-x-3 pt-4 border-t border-gray-200">
              {selectedBackup.status === 'completed' && (
                <>
                  {onDownload && (
                    <Button
                      variant="secondary"
                      onClick={() => {
                        onDownload(selectedBackup.id);
                        setSelectedBackup(null);
                      }}
                    >
                      Download
                    </Button>
                  )}
                  <Button
                    variant="primary"
                    onClick={() => {
                      setRestoreConfirm(selectedBackup.id);
                      setSelectedBackup(null);
                    }}
                  >
                    Restore
                  </Button>
                </>
              )}
              <Button
                variant="secondary"
                onClick={() => setSelectedBackup(null)}
              >
                Close
              </Button>
            </div>
          </div>
        </Modal>
      )}

      {/* Delete Confirmation Modal */}
      <Modal
        isOpen={deleteConfirm !== null}
        onClose={() => setDeleteConfirm(null)}
        title="Confirm Delete"
        size="sm"
      >
        <div className="space-y-4">
          <p className="text-gray-600">
            Are you sure you want to delete this backup? This action cannot be undone.
          </p>
          <div className="flex justify-end space-x-3">
            <Button
              variant="secondary"
              onClick={() => setDeleteConfirm(null)}
            >
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={() => {
                if (deleteConfirm) {
                  onDelete(deleteConfirm);
                  setDeleteConfirm(null);
                }
              }}
            >
              Delete
            </Button>
          </div>
        </div>
      </Modal>

      {/* Restore Confirmation Modal */}
      <Modal
        isOpen={restoreConfirm !== null}
        onClose={() => setRestoreConfirm(null)}
        title="Confirm Restore"
        size="md"
      >
        <div className="space-y-4">
          <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
            <div className="flex">
              <div className="flex-shrink-0">
                <svg className="h-5 w-5 text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.732-.833-2.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z" />
                </svg>
              </div>
              <div className="ml-3">
                <h3 className="text-sm font-medium text-yellow-800">
                  Warning: Database Restore
                </h3>
                <div className="mt-2 text-sm text-yellow-700">
                  <p>
                    Restoring this backup will replace all current data in the database. 
                    This action cannot be undone. Make sure you have a recent backup of the current state if needed.
                  </p>
                </div>
              </div>
            </div>
          </div>
          <p className="text-gray-600">
            Are you sure you want to restore from this backup?
          </p>
          <div className="flex justify-end space-x-3">
            <Button
              variant="secondary"
              onClick={() => setRestoreConfirm(null)}
            >
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={() => {
                if (restoreConfirm) {
                  onRestore(restoreConfirm);
                  setRestoreConfirm(null);
                }
              }}
            >
              Restore Database
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
};

export default BackupList;