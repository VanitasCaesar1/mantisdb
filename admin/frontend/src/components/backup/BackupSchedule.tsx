import React, { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Input, Badge, Modal } from '../ui';
import { PlusIcon, RefreshIcon } from '../icons';

export interface BackupScheduleConfig {
  id: string;
  name: string;
  enabled: boolean;
  schedule: string; // cron expression
  type: 'full' | 'incremental' | 'differential';
  compression: boolean;
  encryption: boolean;
  retention_days: number;
  next_run?: Date;
  last_run?: Date;
  last_status?: 'success' | 'failed';
}

export interface BackupScheduleProps {
  schedules: BackupScheduleConfig[];
  loading?: boolean;
  onRefresh: () => void;
  onCreate: (schedule: Omit<BackupScheduleConfig, 'id' | 'next_run' | 'last_run' | 'last_status'>) => void;
  onUpdate: (id: string, schedule: Partial<BackupScheduleConfig>) => void;
  onDelete: (id: string) => void;
  onToggle: (id: string, enabled: boolean) => void;
}

const BackupSchedule: React.FC<BackupScheduleProps> = ({
  schedules,
  loading = false,
  onRefresh,
  onCreate,
  onUpdate,
  onDelete,
  onToggle
}) => {
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingSchedule, setEditingSchedule] = useState<BackupScheduleConfig | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [newSchedule, setNewSchedule] = useState({
    name: '',
    enabled: true,
    schedule: '0 2 * * *', // Daily at 2 AM
    type: 'full' as const,
    compression: true,
    encryption: false,
    retention_days: 30
  });

  const cronPresets = [
    { label: 'Daily at 2 AM', value: '0 2 * * *' },
    { label: 'Weekly on Sunday at 2 AM', value: '0 2 * * 0' },
    { label: 'Monthly on 1st at 2 AM', value: '0 2 1 * *' },
    { label: 'Every 6 hours', value: '0 */6 * * *' },
    { label: 'Every 12 hours', value: '0 */12 * * *' }
  ];

  const parseCronExpression = (cron: string): string => {
    const preset = cronPresets.find(p => p.value === cron);
    if (preset) return preset.label;
    
    // Basic cron parsing for display
    const parts = cron.split(' ');
    if (parts.length === 5) {
      const [minute, hour, day, month, dayOfWeek] = parts;
      
      if (day === '*' && month === '*' && dayOfWeek === '*') {
        return `Daily at ${hour}:${minute.padStart(2, '0')}`;
      }
      if (day === '*' && month === '*' && dayOfWeek !== '*') {
        const days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
        return `Weekly on ${days[parseInt(dayOfWeek)]} at ${hour}:${minute.padStart(2, '0')}`;
      }
      if (day !== '*' && month === '*' && dayOfWeek === '*') {
        return `Monthly on ${day}th at ${hour}:${minute.padStart(2, '0')}`;
      }
    }
    
    return cron;
  };

  const handleCreateSchedule = () => {
    if (!newSchedule.name.trim()) return;
    
    onCreate(newSchedule);
    setNewSchedule({
      name: '',
      enabled: true,
      schedule: '0 2 * * *',
      type: 'full',
      compression: true,
      encryption: false,
      retention_days: 30
    });
    setShowCreateModal(false);
  };

  const handleUpdateSchedule = () => {
    if (!editingSchedule) return;
    
    onUpdate(editingSchedule.id, editingSchedule);
    setEditingSchedule(null);
  };

  const formatRelativeTime = (date: Date): string => {
    const now = new Date();
    const diffMs = date.getTime() - now.getTime();
    const diffHours = Math.round(diffMs / (1000 * 60 * 60));
    const diffDays = Math.round(diffMs / (1000 * 60 * 60 * 24));
    
    if (diffMs < 0) {
      const pastHours = Math.abs(diffHours);
      const pastDays = Math.abs(diffDays);
      if (pastDays > 0) return `${pastDays} day${pastDays > 1 ? 's' : ''} ago`;
      if (pastHours > 0) return `${pastHours} hour${pastHours > 1 ? 's' : ''} ago`;
      return 'Just now';
    }
    
    if (diffDays > 0) return `in ${diffDays} day${diffDays > 1 ? 's' : ''}`;
    if (diffHours > 0) return `in ${diffHours} hour${diffHours > 1 ? 's' : ''}`;
    return 'Soon';
  };

  const enabledSchedules = schedules.filter(s => s.enabled);
  const disabledSchedules = schedules.filter(s => !s.enabled);

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Backup Schedules</CardTitle>
              <p className="text-sm text-gray-600 mt-1">
                {enabledSchedules.length} active • {disabledSchedules.length} disabled
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
                onClick={() => setShowCreateModal(true)}
              >
                <PlusIcon className="w-4 h-4 mr-2" />
                Add Schedule
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {loading && schedules.length === 0 ? (
            <div className="text-center py-8">
              <div className="animate-spin w-8 h-8 border-2 border-mantis-600 border-t-transparent rounded-full mx-auto mb-4"></div>
              <p className="text-gray-600">Loading schedules...</p>
            </div>
          ) : schedules.length === 0 ? (
            <div className="text-center py-8">
              <div className="text-gray-400 mb-4">
                <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <h3 className="text-lg font-medium text-gray-900 mb-2">No backup schedules</h3>
              <p className="text-gray-600 mb-4">
                Set up automated backup schedules to ensure regular data protection.
              </p>
              <Button variant="primary" onClick={() => setShowCreateModal(true)}>
                <PlusIcon className="w-4 h-4 mr-2" />
                Create First Schedule
              </Button>
            </div>
          ) : (
            <div className="space-y-4">
              {schedules.map((schedule) => (
                <div
                  key={schedule.id}
                  className={`border rounded-lg p-4 ${
                    schedule.enabled ? 'border-gray-200 bg-white' : 'border-gray-200 bg-gray-50 opacity-75'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-3">
                      <label className="flex items-center">
                        <input
                          type="checkbox"
                          checked={schedule.enabled}
                          onChange={(e) => onToggle(schedule.id, e.target.checked)}
                          className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
                        />
                      </label>
                      <div className="flex-1">
                        <div className="flex items-center space-x-2 mb-1">
                          <h4 className="text-sm font-medium text-gray-900">
                            {schedule.name}
                          </h4>
                          <Badge variant={schedule.enabled ? 'success' : 'default'} size="sm">
                            {schedule.enabled ? 'Active' : 'Disabled'}
                          </Badge>
                          <Badge variant="info" size="sm">
                            {schedule.type}
                          </Badge>
                          {schedule.compression && (
                            <Badge variant="default" size="sm">Compressed</Badge>
                          )}
                          {schedule.encryption && (
                            <Badge variant="warning" size="sm">Encrypted</Badge>
                          )}
                        </div>
                        <div className="flex items-center space-x-4 text-sm text-gray-600">
                          <span>{parseCronExpression(schedule.schedule)}</span>
                          <span>Retention: {schedule.retention_days} days</span>
                          {schedule.next_run && (
                            <span>Next: {formatRelativeTime(schedule.next_run)}</span>
                          )}
                        </div>
                        {schedule.last_run && (
                          <div className="flex items-center space-x-2 mt-1 text-xs text-gray-500">
                            <span>Last run: {formatRelativeTime(schedule.last_run)}</span>
                            {schedule.last_status && (
                              <Badge 
                                variant={schedule.last_status === 'success' ? 'success' : 'danger'} 
                                size="sm"
                              >
                                {schedule.last_status}
                              </Badge>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center space-x-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditingSchedule(schedule)}
                      >
                        Edit
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setDeleteConfirm(schedule.id)}
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

      {/* Create Schedule Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        title="Create Backup Schedule"
        size="lg"
      >
        <div className="space-y-4">
          <Input
            label="Schedule Name"
            value={newSchedule.name}
            onChange={(e) => setNewSchedule(prev => ({ ...prev, name: e.target.value }))}
            placeholder="Enter schedule name"
          />

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Schedule
            </label>
            <select
              value={newSchedule.schedule}
              onChange={(e) => setNewSchedule(prev => ({ ...prev, schedule: e.target.value }))}
              className="w-full border border-gray-300 rounded px-3 py-2"
            >
              {cronPresets.map(preset => (
                <option key={preset.value} value={preset.value}>
                  {preset.label}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Backup Type
            </label>
            <select
              value={newSchedule.type}
              onChange={(e) => setNewSchedule(prev => ({ ...prev, type: e.target.value as any }))}
              className="w-full border border-gray-300 rounded px-3 py-2"
            >
              <option value="full">Full Backup</option>
              <option value="incremental">Incremental Backup</option>
              <option value="differential">Differential Backup</option>
            </select>
          </div>

          <div className="space-y-3">
            <label className="flex items-center space-x-2">
              <input
                type="checkbox"
                checked={newSchedule.compression}
                onChange={(e) => setNewSchedule(prev => ({ ...prev, compression: e.target.checked }))}
                className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
              />
              <span className="text-sm text-gray-700">Enable compression</span>
            </label>

            <label className="flex items-center space-x-2">
              <input
                type="checkbox"
                checked={newSchedule.encryption}
                onChange={(e) => setNewSchedule(prev => ({ ...prev, encryption: e.target.checked }))}
                className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
              />
              <span className="text-sm text-gray-700">Enable encryption</span>
            </label>
          </div>

          <Input
            label="Retention Period (Days)"
            type="number"
            value={newSchedule.retention_days}
            onChange={(e) => setNewSchedule(prev => ({ ...prev, retention_days: parseInt(e.target.value) }))}
            min={1}
            max={365}
          />

          <div className="flex justify-end space-x-3 pt-4 border-t border-gray-200">
            <Button
              variant="secondary"
              onClick={() => setShowCreateModal(false)}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              onClick={handleCreateSchedule}
              disabled={!newSchedule.name.trim()}
            >
              Create Schedule
            </Button>
          </div>
        </div>
      </Modal>

      {/* Edit Schedule Modal */}
      {editingSchedule && (
        <Modal
          isOpen={true}
          onClose={() => setEditingSchedule(null)}
          title="Edit Backup Schedule"
          size="lg"
        >
          <div className="space-y-4">
            <Input
              label="Schedule Name"
              value={editingSchedule.name}
              onChange={(e) => setEditingSchedule(prev => prev ? ({ ...prev, name: e.target.value }) : null)}
              placeholder="Enter schedule name"
            />

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Schedule
              </label>
              <select
                value={editingSchedule.schedule}
                onChange={(e) => setEditingSchedule(prev => prev ? ({ ...prev, schedule: e.target.value }) : null)}
                className="w-full border border-gray-300 rounded px-3 py-2"
              >
                {cronPresets.map(preset => (
                  <option key={preset.value} value={preset.value}>
                    {preset.label}
                  </option>
                ))}
              </select>
            </div>

            <div className="flex justify-end space-x-3 pt-4 border-t border-gray-200">
              <Button
                variant="secondary"
                onClick={() => setEditingSchedule(null)}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                onClick={handleUpdateSchedule}
              >
                Update Schedule
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
            Are you sure you want to delete this backup schedule? This action cannot be undone.
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
    </>
  );
};

export default BackupSchedule;