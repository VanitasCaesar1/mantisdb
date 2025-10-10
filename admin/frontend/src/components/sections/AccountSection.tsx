import { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent, Button, Input, Modal } from '../ui';
import { useAuth } from '../../contexts/AuthContext';

export function AccountSection() {
  const { user } = useAuth();
  const [showPasswordModal, setShowPasswordModal] = useState(false);
  const [showCreateUserModal, setShowCreateUserModal] = useState(false);
  const [passwordData, setPasswordData] = useState({
    currentPassword: '',
    newPassword: '',
    confirmPassword: ''
  });
  const [newUser, setNewUser] = useState({
    email: '',
    password: '',
    confirmPassword: '',
    role: 'user'
  });
  const [profileData, setProfileData] = useState({
    email: user?.email || '',
    role: user?.role || ''
  });
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const handlePasswordChange = async () => {
    if (passwordData.newPassword !== passwordData.confirmPassword) {
      setMessage({ type: 'error', text: 'New passwords do not match' });
      return;
    }

    if (passwordData.newPassword.length < 6) {
      setMessage({ type: 'error', text: 'Password must be at least 6 characters' });
      return;
    }

    try {
      const token = localStorage.getItem('mantisdb_token');
      const response = await fetch('/api/auth/change-password', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          current_password: passwordData.currentPassword,
          new_password: passwordData.newPassword
        })
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to change password');
      }

      setMessage({ type: 'success', text: 'Password changed successfully' });
      setShowPasswordModal(false);
      setPasswordData({ currentPassword: '', newPassword: '', confirmPassword: '' });
      setTimeout(() => setMessage(null), 5000);
    } catch (err) {
      setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to change password' });
    }
  };

  const handleCreateUser = async () => {
    if (newUser.password !== newUser.confirmPassword) {
      setMessage({ type: 'error', text: 'Passwords do not match' });
      return;
    }

    if (newUser.password.length < 6) {
      setMessage({ type: 'error', text: 'Password must be at least 6 characters' });
      return;
    }

    try {
      const token = localStorage.getItem('mantisdb_token');
      const response = await fetch('/api/auth/create-user', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          email: newUser.email,
          password: newUser.password,
          role: newUser.role
        })
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to create user');
      }

      setMessage({ type: 'success', text: 'User created successfully' });
      setShowCreateUserModal(false);
      setNewUser({ email: '', password: '', confirmPassword: '', role: 'user' });
      setTimeout(() => setMessage(null), 5000);
    } catch (err) {
      setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to create user' });
    }
  };

  const handleUpdateProfile = async () => {
    try {
      const token = localStorage.getItem('mantisdb_token');
      const response = await fetch('/api/auth/update-profile', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          email: profileData.email
        })
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to update profile');
      }

      setMessage({ type: 'success', text: 'Profile updated successfully' });
      setTimeout(() => setMessage(null), 5000);
    } catch (err) {
      setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to update profile' });
    }
  };

  return (
    <div className="space-y-6">
      {/* Message Banner */}
      {message && (
        <div className={`p-4 rounded-lg ${
          message.type === 'success' ? 'bg-green-50 text-green-800 border border-green-200' : 'bg-red-50 text-red-800 border border-red-200'
        }`}>
          {message.text}
        </div>
      )}

      {/* Profile Information */}
      <Card>
        <CardHeader>
          <CardTitle>Profile Information</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Email
              </label>
              <Input
                type="email"
                value={profileData.email}
                onChange={(e) => setProfileData({ ...profileData, email: e.target.value })}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Role
              </label>
              <Input
                type="text"
                value={profileData.role}
                disabled
                className="bg-gray-100 cursor-not-allowed"
              />
              <p className="text-xs text-gray-500 mt-1">Role cannot be changed by yourself</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                User ID
              </label>
              <Input
                type="text"
                value={user?.id || ''}
                disabled
                className="bg-gray-100 cursor-not-allowed"
              />
            </div>
            <Button onClick={handleUpdateProfile}>
              Update Profile
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Password Management */}
      <Card>
        <CardHeader>
          <CardTitle>Security</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div className="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
              <div>
                <h4 className="font-medium text-gray-900">Password</h4>
                <p className="text-sm text-gray-600">••••••••</p>
              </div>
              <Button variant="secondary" onClick={() => setShowPasswordModal(true)}>
                Change Password
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* User Management (Admin Only) */}
      {user?.role === 'admin' && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>User Management</CardTitle>
              <Button onClick={() => setShowCreateUserModal(true)}>
                + Create User
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <div className="text-sm text-gray-600">
              <p>As an admin, you can create new users and manage their access.</p>
              <p className="mt-2">Click "Create User" to add a new account.</p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Change Password Modal */}
      {showPasswordModal && (
        <Modal isOpen={showPasswordModal} onClose={() => setShowPasswordModal(false)} title="Change Password">
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Current Password
              </label>
              <Input
                type="password"
                value={passwordData.currentPassword}
                onChange={(e) => setPasswordData({ ...passwordData, currentPassword: e.target.value })}
                placeholder="Enter current password"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                New Password
              </label>
              <Input
                type="password"
                value={passwordData.newPassword}
                onChange={(e) => setPasswordData({ ...passwordData, newPassword: e.target.value })}
                placeholder="Enter new password"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Confirm New Password
              </label>
              <Input
                type="password"
                value={passwordData.confirmPassword}
                onChange={(e) => setPasswordData({ ...passwordData, confirmPassword: e.target.value })}
                placeholder="Confirm new password"
              />
            </div>
            <div className="flex justify-end space-x-2 pt-4">
              <Button variant="secondary" onClick={() => setShowPasswordModal(false)}>
                Cancel
              </Button>
              <Button onClick={handlePasswordChange}>
                Change Password
              </Button>
            </div>
          </div>
        </Modal>
      )}

      {/* Create User Modal */}
      {showCreateUserModal && (
        <Modal isOpen={showCreateUserModal} onClose={() => setShowCreateUserModal(false)} title="Create New User">
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Email
              </label>
              <Input
                type="email"
                value={newUser.email}
                onChange={(e) => setNewUser({ ...newUser, email: e.target.value })}
                placeholder="user@example.com"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Password
              </label>
              <Input
                type="password"
                value={newUser.password}
                onChange={(e) => setNewUser({ ...newUser, password: e.target.value })}
                placeholder="Enter password"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Confirm Password
              </label>
              <Input
                type="password"
                value={newUser.confirmPassword}
                onChange={(e) => setNewUser({ ...newUser, confirmPassword: e.target.value })}
                placeholder="Confirm password"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Role
              </label>
              <select
                value={newUser.role}
                onChange={(e) => setNewUser({ ...newUser, role: e.target.value })}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-500"
              >
                <option value="user">User</option>
                <option value="admin">Admin</option>
              </select>
            </div>
            <div className="flex justify-end space-x-2 pt-4">
              <Button variant="secondary" onClick={() => setShowCreateUserModal(false)}>
                Cancel
              </Button>
              <Button onClick={handleCreateUser}>
                Create User
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
