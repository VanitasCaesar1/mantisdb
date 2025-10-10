import { useEffect, useState } from 'react';
import { useAuth } from '../../contexts/AuthContext';
import { Card, CardHeader, CardTitle, CardContent, Button, Input } from '../ui';

export function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [firstRun, setFirstRun] = useState<boolean | null>(null);
  const { login } = useAuth();

  useEffect(() => {
    // Detect first-run state
    (async () => {
      try {
        const res = await fetch('/api/auth/first-run/status');
        if (res.ok) {
          const data = await res.json();
          setFirstRun(!!data.first_run);
        } else {
          setFirstRun(false);
        }
      } catch {
        setFirstRun(false);
      }
    })();
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      if (firstRun) {
        // Create initial admin, then login
        const resp = await fetch('/api/auth/first-run/create', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email, password }),
        });
        if (!resp.ok) {
          const err = await resp.json().catch(() => ({}));
          throw new Error(err.error || 'Failed to create admin');
        }
        await login(email, password);
      } else {
        await login(email, password);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-mantis-50 to-mantis-100 flex items-center justify-center p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <div className="text-center">
            <div className="w-16 h-16 bg-mantis-600 rounded-xl flex items-center justify-center mx-auto mb-4">
              <svg className="w-10 h-10 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
              </svg>
            </div>
            <CardTitle className="text-2xl">MantisDB Admin</CardTitle>
            <p className="text-gray-600 mt-2">
              {firstRun ? 'Create your admin account' : 'Sign in to access the dashboard'}
            </p>
          </div>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg text-sm">
                {error}
              </div>
            )}

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Email
              </label>
              <Input
                type="email"
                value={email}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEmail(e.target.value)}
                placeholder="admin@example.com"
                required
                autoComplete={firstRun ? 'email' : 'current-email'}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Password
              </label>
              <Input
                type="password"
                value={password}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setPassword(e.target.value)}
                placeholder="••••••••"
                required
                autoComplete={firstRun ? 'new-password' : 'current-password'}
              />
            </div>

            <Button 
              type="submit" 
              className="w-full" 
              disabled={loading || firstRun === null}
            >
              {loading ? (firstRun ? 'Creating...' : 'Signing in...') : (firstRun ? 'Create Admin' : 'Sign In')}
            </Button>

            {!firstRun && (
              <div className="text-center text-sm text-gray-600 mt-4">
                <p>Default credentials:</p>
                <p className="font-mono text-xs mt-1">admin@mantisdb.io / admin123</p>
              </div>
            )}
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
