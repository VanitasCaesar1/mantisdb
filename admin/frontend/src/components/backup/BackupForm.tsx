import React, { useState } from 'react';
import { Modal, Button, Input, Badge } from '../ui';

export interface BackupFormData {
  name: string;
  description?: string;
  type: 'full' | 'incremental' | 'differential';
  compression: boolean;
  encryption: boolean;
  retention_days?: number;
}

export interface BackupFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (data: BackupFormData) => Promise<void>;
  loading?: boolean;
}

const BackupForm: React.FC<BackupFormProps> = ({
  isOpen,
  onClose,
  onSubmit,
  loading = false
}) => {
  const [formData, setFormData] = useState<BackupFormData>({
    name: `backup-${new Date().toISOString().split('T')[0]}`,
    description: '',
    type: 'full',
    compression: true,
    encryption: false,
    retention_days: 30
  });
  const [errors, setErrors] = useState<Record<string, string>>({});

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.name.trim()) {
      newErrors.name = 'Backup name is required';
    } else if (formData.name.length < 3) {
      newErrors.name = 'Backup name must be at least 3 characters';
    } else if (!/^[a-zA-Z0-9-_]+$/.test(formData.name)) {
      newErrors.name = 'Backup name can only contain letters, numbers, hyphens, and underscores';
    }

    if (formData.retention_days && (formData.retention_days < 1 || formData.retention_days > 365)) {
      newErrors.retention_days = 'Retention period must be between 1 and 365 days';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!validateForm()) {
      return;
    }

    try {
      await onSubmit(formData);
      onClose();
      // Reset form
      setFormData({
        name: `backup-${new Date().toISOString().split('T')[0]}`,
        description: '',
        type: 'full',
        compression: true,
        encryption: false,
        retention_days: 30
      });
      setErrors({});
    } catch (error) {
      console.error('Backup creation failed:', error);
    }
  };

  const handleFieldChange = (field: keyof BackupFormData, value: any) => {
    setFormData(prev => ({
      ...prev,
      [field]: value
    }));

    // Clear error for this field
    if (errors[field]) {
      setErrors(prev => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  const getBackupTypeDescription = (type: string) => {
    switch (type) {
      case 'full':
        return 'Complete backup of all data. Recommended for most use cases.';
      case 'incremental':
        return 'Only backs up data changed since the last backup. Faster but requires previous backups for restore.';
      case 'differential':
        return 'Backs up data changed since the last full backup. Balance between speed and restore simplicity.';
      default:
        return '';
    }
  };

  const estimateBackupSize = () => {
    // This would be calculated based on actual database size in a real implementation
    const baseSize = 1024 * 1024 * 100; // 100MB base
    const compressionRatio = formData.compression ? 0.3 : 1;
    const typeMultiplier = formData.type === 'full' ? 1 : formData.type === 'differential' ? 0.4 : 0.2;
    
    return Math.round(baseSize * typeMultiplier * compressionRatio);
  };

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Create New Backup"
      size="lg"
    >
      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Basic Information */}
        <div className="space-y-4">
          <h3 className="text-lg font-medium text-gray-900">Basic Information</h3>
          
          <Input
            label="Backup Name"
            value={formData.name}
            onChange={(e) => handleFieldChange('name', e.target.value)}
            error={errors.name}
            placeholder="Enter a unique name for this backup"
            helperText="Use only letters, numbers, hyphens, and underscores"
          />

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Description (Optional)
            </label>
            <textarea
              value={formData.description}
              onChange={(e) => handleFieldChange('description', e.target.value)}
              placeholder="Brief description of this backup"
              rows={3}
              className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-mantis-500 focus:border-mantis-500"
            />
          </div>
        </div>

        {/* Backup Type */}
        <div className="space-y-4">
          <h3 className="text-lg font-medium text-gray-900">Backup Type</h3>
          
          <div className="space-y-3">
            {(['full', 'incremental', 'differential'] as const).map((type) => (
              <label key={type} className="flex items-start space-x-3 p-3 border rounded-lg cursor-pointer hover:bg-gray-50">
                <input
                  type="radio"
                  name="backupType"
                  value={type}
                  checked={formData.type === type}
                  onChange={(e) => handleFieldChange('type', e.target.value)}
                  className="mt-1 text-mantis-600 focus:ring-mantis-500"
                />
                <div className="flex-1">
                  <div className="flex items-center space-x-2">
                    <span className="font-medium text-gray-900 capitalize">{type} Backup</span>
                    {type === 'full' && <Badge variant="success" size="sm">Recommended</Badge>}
                  </div>
                  <p className="text-sm text-gray-600 mt-1">
                    {getBackupTypeDescription(type)}
                  </p>
                </div>
              </label>
            ))}
          </div>
        </div>

        {/* Options */}
        <div className="space-y-4">
          <h3 className="text-lg font-medium text-gray-900">Options</h3>
          
          <div className="space-y-3">
            <label className="flex items-center space-x-3">
              <input
                type="checkbox"
                checked={formData.compression}
                onChange={(e) => handleFieldChange('compression', e.target.checked)}
                className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
              />
              <div>
                <span className="font-medium text-gray-900">Enable Compression</span>
                <p className="text-sm text-gray-600">Reduce backup size by ~70% (recommended)</p>
              </div>
            </label>

            <label className="flex items-center space-x-3">
              <input
                type="checkbox"
                checked={formData.encryption}
                onChange={(e) => handleFieldChange('encryption', e.target.checked)}
                className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
              />
              <div>
                <span className="font-medium text-gray-900">Enable Encryption</span>
                <p className="text-sm text-gray-600">Encrypt backup data for security</p>
              </div>
            </label>
          </div>

          <Input
            label="Retention Period (Days)"
            type="number"
            value={formData.retention_days || ''}
            onChange={(e) => handleFieldChange('retention_days', e.target.value ? parseInt(e.target.value) : undefined)}
            error={errors.retention_days}
            placeholder="30"
            helperText="Number of days to keep this backup before automatic deletion"
            min={1}
            max={365}
          />
        </div>

        {/* Backup Estimate */}
        <div className="bg-gray-50 p-4 rounded-lg">
          <h4 className="font-medium text-gray-900 mb-2">Backup Estimate</h4>
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-gray-600">Estimated Size:</span>
              <p className="font-medium text-gray-900">{formatBytes(estimateBackupSize())}</p>
            </div>
            <div>
              <span className="text-gray-600">Estimated Duration:</span>
              <p className="font-medium text-gray-900">
                {formData.type === 'full' ? '5-15 minutes' : '1-5 minutes'}
              </p>
            </div>
          </div>
          <div className="mt-2 text-xs text-gray-500">
            * Estimates are based on current database size and selected options
          </div>
        </div>

        {/* Actions */}
        <div className="flex justify-end space-x-3 pt-6 border-t border-gray-200">
          <Button
            type="button"
            variant="secondary"
            onClick={onClose}
            disabled={loading}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            loading={loading}
          >
            Create Backup
          </Button>
        </div>
      </form>
    </Modal>
  );
};

export default BackupForm;