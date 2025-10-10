import React, { useState } from 'react';

interface SupabaseLayoutProps {
  children: React.ReactNode;
  activeSection: string;
  onSectionChange: (section: string) => void;
}

const navigationItems = [
  {
    category: 'Database',
    items: [
      { id: 'table-editor', label: 'Table Editor', icon: '📊' },
      { id: 'sql-editor', label: 'SQL Editor', icon: '⚡' },
      { id: 'data-browser', label: 'Data Browser', icon: '🔍' },
    ]
  },
  {
    category: 'Data Models',
    items: [
      { id: 'keyvalue', label: 'Key-Value', icon: '🔑' },
      { id: 'document', label: 'Documents', icon: '📄' },
      { id: 'columnar', label: 'Columnar', icon: '📈' },
    ]
  },
  {
    category: 'Configuration',
    items: [
      { id: 'schema', label: 'Schema', icon: '🏗️' },
      { id: 'rls', label: 'RLS Policies', icon: '🔒' },
      { id: 'auth', label: 'Authentication', icon: '👤' },
      { id: 'storage', label: 'Storage', icon: '💾' },
    ]
  },
  {
    category: 'Operations',
    items: [
      { id: 'monitoring', label: 'Monitoring', icon: '📡', badge: 'Live' },
      { id: 'logs', label: 'Logs', icon: '📝' },
      { id: 'backups', label: 'Backups', icon: '💼' },
      { id: 'settings', label: 'Settings', icon: '⚙️' },
    ]
  }
];

export const SupabaseLayout: React.FC<SupabaseLayoutProps> = ({
  children,
  activeSection,
  onSectionChange,
}) => {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside
        className={`bg-gray-900 text-white transition-all duration-300 ${
          sidebarCollapsed ? 'w-16' : 'w-64'
        } flex flex-col`}
      >
        {/* Logo */}
        <div className="h-16 flex items-center justify-between px-4 border-b border-gray-800">
          {!sidebarCollapsed && (
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 bg-mantis-600 rounded flex items-center justify-center font-bold">
                M
              </div>
              <span className="font-semibold text-lg">MantisDB</span>
            </div>
          )}
          <button
            onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
            className="p-1 hover:bg-gray-800 rounded"
          >
            {sidebarCollapsed ? '→' : '←'}
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto py-4">
          {navigationItems.map((category, idx) => (
            <div key={idx} className="mb-6">
              {!sidebarCollapsed && (
                <div className="px-4 mb-2 text-xs font-semibold text-gray-400 uppercase tracking-wider">
                  {category.category}
                </div>
              )}
              <div className="space-y-1">
                {category.items.map(item => (
                  <button
                    key={item.id}
                    onClick={() => onSectionChange(item.id)}
                    className={`w-full flex items-center gap-3 px-4 py-2 text-sm transition-colors ${
                      activeSection === item.id
                        ? 'bg-mantis-600 text-white'
                        : 'text-gray-300 hover:bg-gray-800 hover:text-white'
                    }`}
                    title={sidebarCollapsed ? item.label : ''}
                  >
                    <span className="text-lg">{item.icon}</span>
                    {!sidebarCollapsed && (
                      <>
                        <span className="flex-1 text-left">{item.label}</span>
                        {item.badge && (
                          <span className="px-2 py-0.5 text-xs bg-green-500 text-white rounded-full">
                            {item.badge}
                          </span>
                        )}
                      </>
                    )}
                  </button>
                ))}
              </div>
            </div>
          ))}
        </nav>

        {/* User Section */}
        <div className="border-t border-gray-800 p-4">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-gray-700 rounded-full flex items-center justify-center">
              👤
            </div>
            {!sidebarCollapsed && (
              <div className="flex-1">
                <div className="text-sm font-medium">Admin</div>
                <div className="text-xs text-gray-400">admin@mantisdb.io</div>
              </div>
            )}
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 overflow-hidden flex flex-col">
        {children}
      </main>
    </div>
  );
};
