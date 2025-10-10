import React, { useState, useEffect } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button } from '../ui';

interface Policy {
  name: string;
  table: string;
  command: 'Select' | 'Insert' | 'Update' | 'Delete' | 'All';
  permission: 'Permissive' | 'Restrictive';
  roles: string[];
  using_expr?: string;
  with_check_expr?: string;
  enabled: boolean;
}

export const RLSPolicyManager: React.FC = () => {
  const [tables, setTables] = useState<string[]>([]);
  const [selectedTable, setSelectedTable] = useState<string>('');
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [rlsEnabled, setRlsEnabled] = useState(false);
  const [showAddPolicy, setShowAddPolicy] = useState(false);
  const [loading, setLoading] = useState(false);

  // New policy form state
  const [newPolicy, setNewPolicy] = useState<Policy>({
    name: '',
    table: '',
    command: 'All',
    permission: 'Permissive',
    roles: [],
    using_expr: 'true',
    with_check_expr: '',
    enabled: true,
  });

  useEffect(() => {
    loadTables();
  }, []);

  useEffect(() => {
    if (selectedTable) {
      loadPolicies();
      checkRLSStatus();
    }
  }, [selectedTable]);

  const loadTables = async () => {
    try {
      const response = await fetch('http://localhost:8081/api/tables');
      const data = await response.json();
      if (data.tables) {
        setTables(data.tables.map((t: any) => t.name || t));
      }
    } catch (error) {
      console.error('Error loading tables:', error);
    }
  };

  const loadPolicies = async () => {
    try {
      const response = await fetch(
        `http://localhost:8081/api/rls/policies?table=${selectedTable}`
      );
      const data = await response.json();
      if (data.success && data.data) {
        setPolicies(data.data.policies || []);
      }
    } catch (error) {
      console.error('Error loading policies:', error);
    }
  };

  const checkRLSStatus = async () => {
    try {
      const response = await fetch(
        `http://localhost:8081/api/rls/status?table=${selectedTable}`
      );
      const data = await response.json();
      if (data.success && data.data) {
        setRlsEnabled(data.data.enabled);
      }
    } catch (error) {
      console.error('Error checking RLS status:', error);
    }
  };

  const toggleRLS = async () => {
    setLoading(true);
    try {
      const endpoint = rlsEnabled ? '/api/rls/disable' : '/api/rls/enable';
      const response = await fetch(`http://localhost:8081${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ table: selectedTable }),
      });

      if (response.ok) {
        setRlsEnabled(!rlsEnabled);
      }
    } catch (error) {
      console.error('Error toggling RLS:', error);
    } finally {
      setLoading(false);
    }
  };

  const addPolicy = async () => {
    if (!newPolicy.name || !selectedTable) {
      alert('Policy name and table are required');
      return;
    }

    setLoading(true);
    try {
      const policyData = {
        ...newPolicy,
        table: selectedTable,
        roles: newPolicy.roles.filter(r => r.trim()),
      };

      const response = await fetch('http://localhost:8081/api/rls/policies/add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ policy: policyData }),
      });

      if (response.ok) {
        setShowAddPolicy(false);
        setNewPolicy({
          name: '',
          table: '',
          command: 'All',
          permission: 'Permissive',
          roles: [],
          using_expr: 'true',
          with_check_expr: '',
          enabled: true,
        });
        loadPolicies();
      }
    } catch (error) {
      console.error('Error adding policy:', error);
    } finally {
      setLoading(false);
    }
  };

  const removePolicy = async (policyName: string) => {
    if (!confirm(`Delete policy "${policyName}"?`)) return;

    setLoading(true);
    try {
      const response = await fetch('http://localhost:8081/api/rls/policies/remove', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          table: selectedTable,
          policy_name: policyName,
        }),
      });

      if (response.ok) {
        loadPolicies();
      }
    } catch (error) {
      console.error('Error removing policy:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Row Level Security (RLS) Policies</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {/* Table Selection */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Select Table
              </label>
              <select
                value={selectedTable}
                onChange={(e) => setSelectedTable(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-mantis-500"
              >
                <option value="">Choose a table...</option>
                {tables.map(table => (
                  <option key={table} value={table}>{table}</option>
                ))}
              </select>
            </div>

            {selectedTable && (
              <>
                {/* RLS Status Toggle */}
                <div className="flex items-center justify-between p-4 bg-gray-50 rounded">
                  <div>
                    <h3 className="font-medium">RLS Status for {selectedTable}</h3>
                    <p className="text-sm text-gray-600">
                      {rlsEnabled
                        ? 'Row Level Security is enabled for this table'
                        : 'Row Level Security is disabled for this table'}
                    </p>
                  </div>
                  <Button
                    onClick={toggleRLS}
                    variant={rlsEnabled ? 'danger' : 'primary'}
                    disabled={loading}
                  >
                    {rlsEnabled ? 'Disable RLS' : 'Enable RLS'}
                  </Button>
                </div>

                {/* Policies List */}
                <div>
                  <div className="flex justify-between items-center mb-4">
                    <h3 className="text-lg font-medium">Policies</h3>
                    <Button onClick={() => setShowAddPolicy(true)}>
                      + Add Policy
                    </Button>
                  </div>

                  {policies.length === 0 ? (
                    <div className="text-center text-gray-500 py-8 border rounded">
                      No policies defined for this table
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {policies.map((policy) => (
                        <div
                          key={policy.name}
                          className="border rounded p-4 hover:bg-gray-50"
                        >
                          <div className="flex justify-between items-start mb-2">
                            <div>
                              <h4 className="font-medium">{policy.name}</h4>
                              <div className="flex gap-2 mt-1">
                                <span className="text-xs px-2 py-1 bg-blue-100 text-blue-800 rounded">
                                  {policy.command}
                                </span>
                                <span className={`text-xs px-2 py-1 rounded ${
                                  policy.permission === 'Permissive'
                                    ? 'bg-green-100 text-green-800'
                                    : 'bg-orange-100 text-orange-800'
                                }`}>
                                  {policy.permission}
                                </span>
                                <span className={`text-xs px-2 py-1 rounded ${
                                  policy.enabled
                                    ? 'bg-green-100 text-green-800'
                                    : 'bg-gray-100 text-gray-800'
                                }`}>
                                  {policy.enabled ? 'Enabled' : 'Disabled'}
                                </span>
                              </div>
                            </div>
                            <Button
                              variant="danger"
                              size="sm"
                              onClick={() => removePolicy(policy.name)}
                            >
                              Delete
                            </Button>
                          </div>
                          
                          {policy.roles.length > 0 && (
                            <div className="text-sm text-gray-600 mb-2">
                              <strong>Roles:</strong> {policy.roles.join(', ')}
                            </div>
                          )}
                          
                          {policy.using_expr && (
                            <div className="text-sm">
                              <strong className="text-gray-700">USING:</strong>
                              <pre className="mt-1 p-2 bg-gray-100 rounded text-xs overflow-x-auto">
                                {policy.using_expr}
                              </pre>
                            </div>
                          )}
                          
                          {policy.with_check_expr && (
                            <div className="text-sm mt-2">
                              <strong className="text-gray-700">WITH CHECK:</strong>
                              <pre className="mt-1 p-2 bg-gray-100 rounded text-xs overflow-x-auto">
                                {policy.with_check_expr}
                              </pre>
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Add Policy Modal */}
      {showAddPolicy && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card className="w-full max-w-2xl max-h-[90vh] overflow-y-auto">
            <CardHeader>
              <CardTitle>Add New Policy</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Policy Name *
                  </label>
                  <input
                    type="text"
                    value={newPolicy.name}
                    onChange={(e) => setNewPolicy({ ...newPolicy, name: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                    placeholder="e.g., users_select_own_data"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Command
                  </label>
                  <select
                    value={newPolicy.command}
                    onChange={(e) => setNewPolicy({ ...newPolicy, command: e.target.value as any })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                  >
                    <option value="All">All</option>
                    <option value="Select">Select</option>
                    <option value="Insert">Insert</option>
                    <option value="Update">Update</option>
                    <option value="Delete">Delete</option>
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Permission Model
                  </label>
                  <select
                    value={newPolicy.permission}
                    onChange={(e) => setNewPolicy({ ...newPolicy, permission: e.target.value as any })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                  >
                    <option value="Permissive">Permissive (OR logic)</option>
                    <option value="Restrictive">Restrictive (AND logic)</option>
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Roles (comma-separated, leave empty for all roles)
                  </label>
                  <input
                    type="text"
                    value={newPolicy.roles.join(', ')}
                    onChange={(e) => setNewPolicy({
                      ...newPolicy,
                      roles: e.target.value.split(',').map(r => r.trim()).filter(Boolean)
                    })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                    placeholder="e.g., authenticated, admin"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    USING Expression
                  </label>
                  <textarea
                    value={newPolicy.using_expr}
                    onChange={(e) => setNewPolicy({ ...newPolicy, using_expr: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md font-mono text-sm"
                    rows={3}
                    placeholder="e.g., user_id = auth.uid()"
                  />
                  <p className="text-xs text-gray-500 mt-1">
                    Expression to filter which rows are visible/affected
                  </p>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    WITH CHECK Expression (optional)
                  </label>
                  <textarea
                    value={newPolicy.with_check_expr}
                    onChange={(e) => setNewPolicy({ ...newPolicy, with_check_expr: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md font-mono text-sm"
                    rows={3}
                    placeholder="e.g., role = 'user'"
                  />
                  <p className="text-xs text-gray-500 mt-1">
                    Expression to validate inserted/updated rows
                  </p>
                </div>

                <div className="flex justify-end gap-2 pt-4 border-t">
                  <Button
                    variant="secondary"
                    onClick={() => setShowAddPolicy(false)}
                  >
                    Cancel
                  </Button>
                  <Button
                    onClick={addPolicy}
                    disabled={loading || !newPolicy.name}
                  >
                    {loading ? 'Adding...' : 'Add Policy'}
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
