import React, { useState } from 'react';
import Sidebar, { SidebarItem } from './Sidebar';
import Header, { HeaderProps } from './Header';

export interface LayoutProps {
  children: React.ReactNode;
  sidebarItems: SidebarItem[];
  activeSidebarItem: string;
  onSidebarItemClick: (itemId: string) => void;
  headerProps?: Omit<HeaderProps, 'title'> & { title?: string };
}

const Layout: React.FC<LayoutProps> = ({
  children,
  sidebarItems,
  activeSidebarItem,
  onSidebarItemClick,
  headerProps
}) => {
  const [sidebarCollapsed] = useState(false);

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <div className={`transition-all duration-300 ${sidebarCollapsed ? 'w-16' : 'w-64'}`}>
        <Sidebar
          items={sidebarItems}
          activeItem={activeSidebarItem}
          onItemClick={onSidebarItemClick}
        />
      </div>

      {/* Main Content */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Header */}
        {headerProps && (
          <Header
            title={headerProps.title || 'Dashboard'}
            {...headerProps}
          />
        )}

        {/* Content */}
        <main className="flex-1 overflow-y-auto">
          <div className="p-6">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
};

export default Layout;