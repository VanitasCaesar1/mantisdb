import React from 'react';

export interface SidebarItem {
  id: string;
  label: string;
  icon: React.ReactNode;
  path: string;
  badge?: string;
}

export interface SidebarProps {
  items: SidebarItem[];
  activeItem: string;
  onItemClick: (itemId: string) => void;
  className?: string;
}

const Sidebar: React.FC<SidebarProps> = ({
  items,
  activeItem,
  onItemClick,
  className = ''
}) => {
  return (
    <div className={`mantis-sidebar ${className}`}>
      {/* Logo/Header */}
      <div className="p-6 border-b border-mantis-700">
        <div className="flex items-center space-x-3">
          <div className="w-8 h-8 bg-mantis-400 rounded-lg flex items-center justify-center">
            <svg className="w-5 h-5 text-mantis-900" fill="currentColor" viewBox="0 0 20 20">
              <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z" />
            </svg>
          </div>
          <div>
            <h1 className="text-lg font-bold text-white">MantisDB</h1>
            <p className="text-xs text-mantis-300">Admin Dashboard</p>
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="mt-6">
        <ul className="space-y-1">
          {items.map((item) => (
            <li key={item.id}>
              <button
                onClick={() => onItemClick(item.id)}
                className={`mantis-nav-item w-full text-left ${
                  activeItem === item.id ? 'active' : ''
                }`}
              >
                <span className="w-5 h-5 mr-3 flex-shrink-0">
                  {item.icon}
                </span>
                <span className="flex-1">{item.label}</span>
                {item.badge && (
                  <span className="ml-2 px-2 py-0.5 text-xs bg-mantis-600 text-mantis-100 rounded-full">
                    {item.badge}
                  </span>
                )}
              </button>
            </li>
          ))}
        </ul>
      </nav>

      {/* Footer */}
      <div className="absolute bottom-0 left-0 right-0 p-4 border-t border-mantis-700">
        <div className="text-xs text-mantis-400 text-center">
          Version 1.0.0
        </div>
      </div>
    </div>
  );
};

export default Sidebar;