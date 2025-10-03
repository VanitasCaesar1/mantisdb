import React, { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Input, Modal, Badge } from '../ui';
import { SearchIcon, PlusIcon } from '../icons';
import { formatRelativeTime, truncate } from '../../utils';

export interface SavedQuery {
  id: string;
  name: string;
  description?: string;
  query: string;
  tags: string[];
  createdAt: Date;
  updatedAt: Date;
  favorite: boolean;
}

export interface SavedQueriesProps {
  queries: SavedQuery[];
  loading?: boolean;
  onSelectQuery: (query: string) => void;
  onSaveQuery: (query: Omit<SavedQuery, 'id' | 'createdAt' | 'updatedAt'>) => void;
  onDeleteQuery: (id: string) => void;
  onToggleFavorite: (id: string) => void;
}

const SavedQueries: React.FC<SavedQueriesProps> = ({
  queries,
  loading = false,
  onSelectQuery,
  onSaveQuery,
  onDeleteQuery,
  onToggleFavorite
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedTag, setSelectedTag] = useState<string>('');
  const [showFavoritesOnly, setShowFavoritesOnly] = useState(false);
  const [showSaveModal, setShowSaveModal] = useState(false);
  const [newQuery, setNewQuery] = useState({
    name: '',
    description: '',
    query: '',
    tags: [] as string[],
    favorite: false
  });

  // Get all unique tags
  const allTags = Array.from(new Set(queries.flatMap(q => q.tags))).sort();

  const filteredQueries = queries.filter(query => {
    const matchesSearch = 
      query.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      query.description?.toLowerCase().includes(searchTerm.toLowerCase()) ||
      query.query.toLowerCase().includes(searchTerm.toLowerCase());
    
    const matchesTag = !selectedTag || query.tags.includes(selectedTag);
    const matchesFavorite = !showFavoritesOnly || query.favorite;
    
    return matchesSearch && matchesTag && matchesFavorite;
  });

  const handleSaveQuery = () => {
    if (!newQuery.name.trim() || !newQuery.query.trim()) return;
    
    onSaveQuery({
      name: newQuery.name.trim(),
      description: newQuery.description.trim() || undefined,
      query: newQuery.query.trim(),
      tags: newQuery.tags,
      favorite: newQuery.favorite
    });
    
    setNewQuery({
      name: '',
      description: '',
      query: '',
      tags: [],
      favorite: false
    });
    setShowSaveModal(false);
  };

  const addTag = (tag: string) => {
    if (tag.trim() && !newQuery.tags.includes(tag.trim())) {
      setNewQuery(prev => ({
        ...prev,
        tags: [...prev.tags, tag.trim()]
      }));
    }
  };

  const removeTag = (tagToRemove: string) => {
    setNewQuery(prev => ({
      ...prev,
      tags: prev.tags.filter(tag => tag !== tagToRemove)
    }));
  };

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Saved Queries</CardTitle>
              <p className="text-sm text-gray-600 mt-1">
                {filteredQueries.length} of {queries.length} queries
              </p>
            </div>
            <Button
              variant="primary"
              size="sm"
              onClick={() => setShowSaveModal(true)}
            >
              <PlusIcon className="w-4 h-4 mr-2" />
              Save Query
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {/* Filters */}
            <div className="flex items-center space-x-4">
              <div className="flex-1">
                <Input
                  placeholder="Search saved queries..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  leftIcon={<SearchIcon className="w-4 h-4" />}
                />
              </div>
              <div className="flex items-center space-x-2">
                <select
                  value={selectedTag}
                  onChange={(e) => setSelectedTag(e.target.value)}
                  className="border border-gray-300 rounded px-3 py-2 text-sm"
                >
                  <option value="">All Tags</option>
                  {allTags.map(tag => (
                    <option key={tag} value={tag}>{tag}</option>
                  ))}
                </select>
                <label className="flex items-center space-x-2">
                  <input
                    type="checkbox"
                    checked={showFavoritesOnly}
                    onChange={(e) => setShowFavoritesOnly(e.target.checked)}
                    className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
                  />
                  <span className="text-sm text-gray-700">Favorites only</span>
                </label>
              </div>
            </div>

            {/* Queries List */}
            {loading ? (
              <div className="text-center py-8">
                <div className="animate-spin w-8 h-8 border-2 border-mantis-600 border-t-transparent rounded-full mx-auto mb-4"></div>
                <p className="text-gray-600">Loading saved queries...</p>
              </div>
            ) : filteredQueries.length === 0 ? (
              <div className="text-center py-8">
                <div className="text-gray-400 mb-4">
                  <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z" />
                  </svg>
                </div>
                <h3 className="text-lg font-medium text-gray-900 mb-2">No saved queries</h3>
                <p className="text-gray-600 mb-4">
                  {searchTerm || selectedTag || showFavoritesOnly
                    ? 'No queries match your search criteria.'
                    : 'Save your frequently used queries for quick access.'
                  }
                </p>
                {!searchTerm && !selectedTag && !showFavoritesOnly && (
                  <Button variant="primary" onClick={() => setShowSaveModal(true)}>
                    <PlusIcon className="w-4 h-4 mr-2" />
                    Save Your First Query
                  </Button>
                )}
              </div>
            ) : (
              <div className="space-y-3">
                {filteredQueries.map((query) => (
                  <div
                    key={query.id}
                    className="border border-gray-200 rounded-lg p-4 hover:bg-gray-50 transition-colors"
                  >
                    <div className="flex items-start justify-between">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center space-x-2 mb-2">
                          <h4 className="text-sm font-medium text-gray-900">
                            {query.name}
                          </h4>
                          {query.favorite && (
                            <svg className="w-4 h-4 text-yellow-400 fill-current" viewBox="0 0 20 20">
                              <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
                            </svg>
                          )}
                        </div>
                        
                        {query.description && (
                          <p className="text-sm text-gray-600 mb-2">
                            {query.description}
                          </p>
                        )}

                        <div className="mb-3">
                          <code className="text-sm bg-gray-100 p-2 rounded block overflow-x-auto">
                            {truncate(query.query, 150)}
                          </code>
                        </div>

                        <div className="flex items-center space-x-4 text-xs text-gray-500">
                          <span>Updated {formatRelativeTime(query.updatedAt)}</span>
                          {query.tags.length > 0 && (
                            <div className="flex items-center space-x-1">
                              <span>Tags:</span>
                              <div className="flex space-x-1">
                                {query.tags.map(tag => (
                                  <Badge key={tag} variant="default" size="sm">
                                    {tag}
                                  </Badge>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      </div>

                      <div className="flex items-center space-x-2 ml-4">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => onToggleFavorite(query.id)}
                        >
                          {query.favorite ? (
                            <svg className="w-4 h-4 text-yellow-400 fill-current" viewBox="0 0 20 20">
                              <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
                            </svg>
                          ) : (
                            <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
                            </svg>
                          )}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => onSelectQuery(query.query)}
                        >
                          Use Query
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => onDeleteQuery(query.id)}
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
          </div>
        </CardContent>
      </Card>

      {/* Save Query Modal */}
      <Modal
        isOpen={showSaveModal}
        onClose={() => setShowSaveModal(false)}
        title="Save Query"
        size="lg"
      >
        <div className="space-y-4">
          <Input
            label="Query Name"
            value={newQuery.name}
            onChange={(e) => setNewQuery(prev => ({ ...prev, name: e.target.value }))}
            placeholder="Enter a descriptive name for your query"
          />
          
          <Input
            label="Description (Optional)"
            value={newQuery.description}
            onChange={(e) => setNewQuery(prev => ({ ...prev, description: e.target.value }))}
            placeholder="Brief description of what this query does"
          />

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              SQL Query
            </label>
            <textarea
              value={newQuery.query}
              onChange={(e) => setNewQuery(prev => ({ ...prev, query: e.target.value }))}
              placeholder="Enter your SQL query here..."
              className="w-full h-32 px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-mantis-500 focus:border-mantis-500 font-mono text-sm"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Tags
            </label>
            <div className="flex flex-wrap gap-2 mb-2">
              {newQuery.tags.map(tag => (
                <Badge key={tag} variant="default" size="sm">
                  {tag}
                  <button
                    onClick={() => removeTag(tag)}
                    className="ml-1 text-gray-500 hover:text-gray-700"
                  >
                    ×
                  </button>
                </Badge>
              ))}
            </div>
            <Input
              placeholder="Add tags (press Enter)"
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  addTag(e.currentTarget.value);
                  e.currentTarget.value = '';
                }
              }}
            />
          </div>

          <label className="flex items-center space-x-2">
            <input
              type="checkbox"
              checked={newQuery.favorite}
              onChange={(e) => setNewQuery(prev => ({ ...prev, favorite: e.target.checked }))}
              className="rounded border-gray-300 text-mantis-600 focus:ring-mantis-500"
            />
            <span className="text-sm text-gray-700">Mark as favorite</span>
          </label>

          <div className="flex justify-end space-x-3 pt-4 border-t border-gray-200">
            <Button
              variant="secondary"
              onClick={() => setShowSaveModal(false)}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              onClick={handleSaveQuery}
              disabled={!newQuery.name.trim() || !newQuery.query.trim()}
            >
              Save Query
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
};

export default SavedQueries;