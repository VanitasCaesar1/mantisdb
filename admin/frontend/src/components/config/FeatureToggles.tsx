import React, { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Badge, Modal } from '../ui';
import { RefreshIcon, PlusIcon } from '../icons';

export interface FeatureToggle {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
  category: 'performance' | 'security' | 'experimental' | 'maintenance';
  requires_restart: boolean;
  created_at: Date;
  updated_at: Date;
}

export interface FeatureTogglesProps {
  features: FeatureToggle[];
  loading?: boolean;
  onRefresh: () => void;
  onToggle: (featureId: string, enabled: boolean) => Promise<void>;
  onCreate?: (feature: Omit<FeatureToggle, 'id' | 'created_at' | 'updated_at'>) => Promise<void>;
  onDelete?: (featureId: string) => Promise<void>;
}

const FeatureToggles: React.FC<FeatureTogglesProps> = ({
  features,
  loading = false,
  onRefresh,
  onToggle,
  onCreate,
  onDelete
}) => {
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newFeature, setNewFeature] = useState({
    name: '',
    description: '',
    enabled: false,
    category: 'experimental' as const,
    requires_restart: false
  });

  const categories = [
    { id: 'all', label: 'All Features', color: 'default' },
    { id: 'performance', label: 'Performance', color: 'info' },
    { id: 'security', label: 'Security', color: 'warning' },
    { id: 'experimental', label: 'Experimental', color: 'danger' },
    { id: 'maintenance', label: 'Maintenance', color: 'default' }
  ];

  const getCategoryColor = (category: string): 'default' | 'success' | 'warning' | 'danger' | 'info' => {
    const cat = categories.find(c => c.id === category);
    return (cat?.color as 'default' | 'success' | 'warning' | 'danger' | 'info') || 'default';
  };

  const getCategoryIcon = (category: string) => {
    switch (category) {
      case 'performance':
        return (
          <svg className="w-4 h-4 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        );
      case 'security':
        return (
          <svg className="w-4 h-4 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
        );
      case 'experimental':
        return (
          <svg className="w-4 h-4 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z" />
          </svg>
        );
      case 'maintenance':
        return (
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        );
      default:
        return null;
    }
  };

  const filteredFeatures = features.filter(feature => 
    selectedCategory === 'all' || feature.category === selectedCategory
  );

  const enabledCount = features.filter(f => f.enabled).length;
  const restartRequiredCount = features.filter(f => f.enabled && f.requires_restart).length;

  const handleToggle = async (featureId: string, enabled: boolean) => {
    try {
      await onToggle(featureId, enabled);
    } catch (error) {
      console.error('Failed to toggle feature:', error);
    }
  };

  const handleCreate = async () => {
    if (!newFeature.name.trim() || !onCreate) return;
    
    try {
      await onCreate(newFeature);
      setNewFeature({
        name: '',
        description: '',
        enabled: false,
        category: 'experimental',
        requires_restart: false
      });
      setShowCreateModal(false);
    } catch (error) {
      console.error('Failed to create feature:', error);
    }
  };

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Feature Toggles</CardTitle>
              <p className="text-sm text-gray-600 mt-1">
                {enabledCount} of {features.length} features enabled
                {restartRequiredCount > 0 && (
                  <span className="ml-2 text-yellow-600">
                    • {restartRequiredCount} require restart
                  </span>
                )}
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
              {onCreate && (
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => setShowCreateModal(true)}
                >
                  <PlusIcon className="w-4 h-4 mr-2" />
                  Add Feature
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-6">
            {/* Category Filter */}
            <div className="flex flex-wrap gap-2">
              {categories.map((category) => (
                <button
                  key={category.id}
                  onClick={() => setSelectedCategory(category.id)}
                  className={`px-3 py-1 text-sm rounded-full transition-colors ${
                    selectedCategory === category.id
                      ? 'bg-mantis-100 text-mantis-800 border border-mantis-300'
                      : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                  }`}
                >
                  {category.label}
                  {category.id !== 'all' && (
                    <span className="ml-1 text-xs">
                      ({features.filter(f => f.category === category.id).length})
                    </span>
                  )}
                </button>
              ))}
            </div>

            {/* Features List */}
            {loading && features.length === 0 ? (
              <div className="text-center py-8">
                <div className="animate-spin w-8 h-8 border-2 border-mantis-600 border-t-transparent rounded-full mx-auto mb-4"></div>
                <p className="text-gray-600">Loading feature toggles...</p>
              </div>
            ) : filteredFeatures.length === 0 ? (
              <div className="text-center py-8">
                <div className="text-gray-400 mb-4">
                  <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z" />
                  </svg>
                </div>
                <h3 className="text-lg font-medium text-gray-900 mb-2">No features found</h3>
                <p className="text-gray-600">
                  {selectedCategory === 'all' 
                    ? 'No feature toggles are configured.'
                    : `No features in the ${selectedCategory} category.`
                  }
                </p>
              </div>
            ) : (
              <div className="space-y-4">
                {filteredFeatures.map((feature) => (
                  <div
                    key={feature.id}
                    className="border border-gray-200 rounded-lg p-4 hover:bg-gray-50 transition-colors"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-start space-x-3">
                        <div className="flex-shrink-0 mt-1">
                          {getCategoryIcon(feature.category)}
                        </div>
                        <div className="flex-1">
                          <div className="flex items-center space-x-2 mb-1">
                            <h4 className="text-sm font-medium text-gray-900">
                              {feature.name}
                            </h4>
                            <Badge variant={getCategoryColor(feature.category)} size="sm">
                              {feature.category}
                            </Badge>
                            {feature.enabled && (
                              <Badge variant="success" size="sm">
                                Enabled
                              </Badge>
                            )}
                            {feature.requires_restart && feature.enabled && (
                              <Badge variant="warning" size="sm">
                                Restart Required
                              </Badge>
                            )}
                          </div>
                          <p className="text-sm text-gray-600 mb-2">
                            {feature.description}
                          </p>
                          <div className="text-xs text-gray-500">
                            Updated {feature.updated_at.toLocaleDateString()}
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center space-x-3">
                        <label className="flex items-center">
                          <input
                            type="checkbox"
                            checked={feature.enabled}
                            onChange={(e) => handleToggle(feature.id, e.target.checked)}
                            className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
                          />
                          <span className="ml-2 text-sm text-gray-700">
                            {feature.enabled ? 'Enabled' : 'Disabled'}
                          </span>
                        </label>
                        {onDelete && (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => onDelete(feature.id)}
                            className="text-red-600 hover:text-red-700"
                          >
                            Delete
                          </Button>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* Restart Warning */}
            {restartRequiredCount > 0 && (
              <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
                <div className="flex">
                  <div className="flex-shrink-0">
                    <svg className="h-5 w-5 text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.732-.833-2.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z" />
                    </svg>
                  </div>
                  <div className="ml-3">
                    <h3 className="text-sm font-medium text-yellow-800">
                      Restart Required
                    </h3>
                    <div className="mt-2 text-sm text-yellow-700">
                      <p>
                        {restartRequiredCount} feature{restartRequiredCount > 1 ? 's' : ''} require{restartRequiredCount === 1 ? 's' : ''} a server restart to take effect.
                        Please restart the database server when convenient.
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Create Feature Modal */}
      {onCreate && (
        <Modal
          isOpen={showCreateModal}
          onClose={() => setShowCreateModal(false)}
          title="Add Feature Toggle"
          size="lg"
        >
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Feature Name
              </label>
              <input
                type="text"
                value={newFeature.name}
                onChange={(e) => setNewFeature(prev => ({ ...prev, name: e.target.value }))}
                placeholder="Enter feature name"
                className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-mantis-500 focus:border-mantis-500"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Description
              </label>
              <textarea
                value={newFeature.description}
                onChange={(e) => setNewFeature(prev => ({ ...prev, description: e.target.value }))}
                placeholder="Describe what this feature does"
                rows={3}
                className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-mantis-500 focus:border-mantis-500"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Category
              </label>
              <select
                value={newFeature.category}
                onChange={(e) => setNewFeature(prev => ({ ...prev, category: e.target.value as any }))}
                className="w-full border border-gray-300 rounded px-3 py-2"
              >
                <option value="performance">Performance</option>
                <option value="security">Security</option>
                <option value="experimental">Experimental</option>
                <option value="maintenance">Maintenance</option>
              </select>
            </div>

            <div className="space-y-3">
              <label className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  checked={newFeature.enabled}
                  onChange={(e) => setNewFeature(prev => ({ ...prev, enabled: e.target.checked }))}
                  className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
                />
                <span className="text-sm text-gray-700">Enable by default</span>
              </label>

              <label className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  checked={newFeature.requires_restart}
                  onChange={(e) => setNewFeature(prev => ({ ...prev, requires_restart: e.target.checked }))}
                  className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
                />
                <span className="text-sm text-gray-700">Requires restart to take effect</span>
              </label>
            </div>

            <div className="flex justify-end space-x-3 pt-4 border-t border-gray-200">
              <Button
                variant="secondary"
                onClick={() => setShowCreateModal(false)}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                onClick={handleCreate}
                disabled={!newFeature.name.trim()}
              >
                Add Feature
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </>
  );
};

export default FeatureToggles;