import { useState, useEffect } from 'react';
import { Shield, Key, Copy, Check, Info, AlertCircle } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '../ui';
import { getAdminPort } from '../../config/api';

interface AdminUser {
  id: string;
  email: string;
  role: string;
}

export const AuthenticationSection = () => {
  const [activeTab, setActiveTab] = useState<'config' | 'policies' | 'api-keys'>('config');
  const [currentUser, setCurrentUser] = useState<AdminUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [apiBaseUrl, setApiBaseUrl] = useState<string>('');

  // Dynamically detect the admin server port
  useEffect(() => {
    getAdminPort().then(port => {
      setApiBaseUrl(`${window.location.protocol}//${window.location.hostname}:${port}/api`);
    });
  }, []);

  useEffect(() => {
    loadCurrentUser();
  }, []);

  const loadCurrentUser = async () => {
    try {
      const token = localStorage.getItem('auth_token');
      if (!token) {
        setLoading(false);
        return;
      }

      const response = await fetch('/api/auth/verify', {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });

      if (response.ok) {
        const data = await response.json();
        setCurrentUser(data.user);
      }
    } catch (error) {
      console.error('Failed to load user:', error);
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = (text: string, keyName: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(keyName);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const tabs = [
    { id: 'config', label: 'Configuration', icon: Shield },
    { id: 'policies', label: 'Access Policies', icon: Key },
    { id: 'api-keys', label: 'API Keys', icon: Key }
  ];

  const renderConfigTab = () => (
    <div className="space-y-6">
      {/* Info Banner */}
      <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 flex items-start gap-3">
        <Info className="w-5 h-5 text-blue-600 flex-shrink-0 mt-0.5" />
        <div>
          <h3 className="text-sm font-medium text-blue-900 mb-1">OAuth2 Authentication</h3>
          <p className="text-sm text-blue-700">
            MantisDB supports OAuth2 authentication with multiple providers (Google, GitHub, Microsoft). 
            Configure your OAuth2 providers below for secure, enterprise-grade authentication.
          </p>
        </div>
      </div>

      {/* OAuth2 Providers */}
      <Card>
        <CardHeader>
          <CardTitle>OAuth2 Providers</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div className="border rounded-lg p-4">
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 bg-white border rounded-lg flex items-center justify-center">
                    <svg className="w-6 h-6" viewBox="0 0 24 24">
                      <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                      <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                      <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                      <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                    </svg>
                  </div>
                  <div>
                    <h4 className="font-medium text-gray-900">Google</h4>
                    <p className="text-xs text-gray-500">Sign in with Google accounts</p>
                  </div>
                </div>
                <input type="checkbox" className="rounded" />
              </div>
              <div className="space-y-2 text-sm">
                <div>
                  <label className="text-gray-600">Client ID</label>
                  <input 
                    type="text" 
                    placeholder="your-client-id.apps.googleusercontent.com"
                    className="w-full mt-1 px-3 py-2 border rounded-lg text-sm"
                  />
                </div>
                <div>
                  <label className="text-gray-600">Client Secret</label>
                  <input 
                    type="password" 
                    placeholder="••••••••••••••••"
                    className="w-full mt-1 px-3 py-2 border rounded-lg text-sm"
                  />
                </div>
              </div>
            </div>

            <div className="border rounded-lg p-4">
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 bg-gray-900 rounded-lg flex items-center justify-center">
                    <svg className="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 24 24">
                      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                    </svg>
                  </div>
                  <div>
                    <h4 className="font-medium text-gray-900">GitHub</h4>
                    <p className="text-xs text-gray-500">Sign in with GitHub accounts</p>
                  </div>
                </div>
                <input type="checkbox" className="rounded" />
              </div>
              <div className="space-y-2 text-sm">
                <div>
                  <label className="text-gray-600">Client ID</label>
                  <input 
                    type="text" 
                    placeholder="your-github-client-id"
                    className="w-full mt-1 px-3 py-2 border rounded-lg text-sm"
                  />
                </div>
                <div>
                  <label className="text-gray-600">Client Secret</label>
                  <input 
                    type="password" 
                    placeholder="••••••••••••••••"
                    className="w-full mt-1 px-3 py-2 border rounded-lg text-sm"
                  />
                </div>
              </div>
            </div>

            <div className="border rounded-lg p-4">
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 bg-blue-600 rounded-lg flex items-center justify-center">
                    <svg className="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 24 24">
                      <path d="M0 0v11.408h11.408V0H0zm12.594 0v11.408H24V0H12.594zM0 12.594V24h11.408V12.594H0zm12.594 0V24H24V12.594H12.594z"/>
                    </svg>
                  </div>
                  <div>
                    <h4 className="font-medium text-gray-900">Microsoft</h4>
                    <p className="text-xs text-gray-500">Sign in with Microsoft accounts</p>
                  </div>
                </div>
                <input type="checkbox" className="rounded" />
              </div>
              <div className="space-y-2 text-sm">
                <div>
                  <label className="text-gray-600">Client ID</label>
                  <input 
                    type="text" 
                    placeholder="your-microsoft-client-id"
                    className="w-full mt-1 px-3 py-2 border rounded-lg text-sm"
                  />
                </div>
                <div>
                  <label className="text-gray-600">Client Secret</label>
                  <input 
                    type="password" 
                    placeholder="••••••••••••••••"
                    className="w-full mt-1 px-3 py-2 border rounded-lg text-sm"
                  />
                </div>
              </div>
            </div>

            <button className="w-full px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors">
              Save OAuth2 Configuration
            </button>
          </div>
        </CardContent>
      </Card>

      {/* Current User */}
      {currentUser && (
        <Card>
          <CardHeader>
            <CardTitle>Current Admin User</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600">Email</span>
                <span className="text-sm font-medium text-gray-900">{currentUser.email}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600">Role</span>
                <span className="px-2 py-1 text-xs font-medium bg-mantis-100 text-mantis-800 rounded-full">
                  {currentUser.role}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600">User ID</span>
                <span className="text-sm font-mono text-gray-900">{currentUser.id}</span>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Database Access */}
      <Card>
        <CardHeader>
          <CardTitle>Database Access Control</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <span className="text-sm font-medium text-gray-900">Enable Admin API</span>
                <p className="text-xs text-gray-500">Allow admin operations via API</p>
              </div>
              <input type="checkbox" defaultChecked className="rounded" />
            </div>
            <div className="flex items-center justify-between">
              <div>
                <span className="text-sm font-medium text-gray-900">Require Authentication</span>
                <p className="text-xs text-gray-500">All API requests must include valid token</p>
              </div>
              <input type="checkbox" defaultChecked className="rounded" />
            </div>
            <div className="flex items-center justify-between">
              <div>
                <span className="text-sm font-medium text-gray-900">Enable CORS</span>
                <p className="text-xs text-gray-500">Allow cross-origin requests</p>
              </div>
              <input type="checkbox" defaultChecked className="rounded" />
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );

  const renderPoliciesTab = () => (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>API Rate Limiting</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Requests per minute
              </label>
              <input
                type="number"
                defaultValue={100}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Burst limit
              </label>
              <input
                type="number"
                defaultValue={200}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-500"
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Session Policy</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Session Timeout (hours)
              </label>
              <input
                type="number"
                defaultValue={24}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Token Lifetime (days)
              </label>
              <input
                type="number"
                defaultValue={30}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-500"
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>IP Allowlist</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <p className="text-sm text-gray-600">
              Restrict API access to specific IP addresses (one per line)
            </p>
            <textarea
              rows={5}
              placeholder="127.0.0.1&#10;192.168.1.0/24"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-mantis-500 font-mono text-sm"
            />
            <button className="px-4 py-2 bg-mantis-600 text-white rounded-lg hover:bg-mantis-700 transition-colors text-sm">
              Save Allowlist
            </button>
          </div>
        </CardContent>
      </Card>
    </div>
  );

  const renderAPIKeysTab = () => (
    <div className="space-y-6">
      {/* Warning Banner */}
      <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4 flex items-start gap-3">
        <AlertCircle className="w-5 h-5 text-yellow-600 flex-shrink-0 mt-0.5" />
        <div>
          <h3 className="text-sm font-medium text-yellow-900 mb-1">Keep your API keys secure</h3>
          <p className="text-sm text-yellow-700">
            Never share your API keys or commit them to version control. Rotate keys regularly.
          </p>
        </div>
      </div>

      {/* API Endpoints */}
      <Card>
        <CardHeader>
          <CardTitle>API Endpoints</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Base URL</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={apiBaseUrl || 'Loading...'}
                  readOnly
                  className="flex-1 px-3 py-2 border border-gray-300 rounded-lg bg-gray-50 font-mono text-sm"
                />
                <button
                  onClick={() => copyToClipboard(apiBaseUrl, 'base-url')}
                  disabled={!apiBaseUrl}
                  className="px-3 py-2 border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
                  title="Copy to clipboard"
                >
                  {copiedKey === 'base-url' ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4" />}
                </button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Auth Token */}
      <Card>
        <CardHeader>
          <CardTitle>Authentication Token</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Current Token</label>
              <div className="flex gap-2">
                <input
                  type="password"
                  value={localStorage.getItem('auth_token') || 'No token found'}
                  readOnly
                  className="flex-1 px-3 py-2 border border-gray-300 rounded-lg bg-gray-50 font-mono text-sm"
                />
                <button
                  onClick={() => copyToClipboard(localStorage.getItem('auth_token') || '', 'auth-token')}
                  className="px-3 py-2 border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
                  title="Copy to clipboard"
                >
                  {copiedKey === 'auth-token' ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4" />}
                </button>
              </div>
              <p className="text-xs text-gray-500 mt-1">
                Use this token in the Authorization header: Bearer [token]
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Example Usage */}
      <Card>
        <CardHeader>
          <CardTitle>Example API Usage</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">cURL Example</label>
              <div className="bg-gray-900 text-gray-100 p-4 rounded-lg font-mono text-sm overflow-x-auto">
                <pre>{`curl -X GET \\
  ${apiBaseUrl}/stats \\
  -H 'Authorization: Bearer YOUR_TOKEN'`}</pre>
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">JavaScript Example</label>
              <div className="bg-gray-900 text-gray-100 p-4 rounded-lg font-mono text-sm overflow-x-auto">
                <pre>{`fetch('${apiBaseUrl}/stats', {
  headers: {
    'Authorization': 'Bearer YOUR_TOKEN'
  }
})
.then(res => res.json())
.then(data => console.log(data));`}</pre>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-mantis-600 mx-auto"></div>
          <p className="text-gray-600 mt-4">Loading...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Authentication & Security</h1>
        <p className="text-gray-600 mt-1">Configure API access, security policies, and authentication settings</p>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200">
        <nav className="-mb-px flex space-x-8">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as any)}
                className={`flex items-center gap-2 py-4 px-1 border-b-2 font-medium text-sm transition-colors ${
                  activeTab === tab.id
                    ? 'border-mantis-600 text-mantis-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                <Icon className="w-4 h-4" />
                {tab.label}
              </button>
            );
          })}
        </nav>
      </div>

      {/* Tab Content */}
      <div>
        {activeTab === 'config' && renderConfigTab()}
        {activeTab === 'policies' && renderPoliciesTab()}
        {activeTab === 'api-keys' && renderAPIKeysTab()}
      </div>
    </div>
  );
};
