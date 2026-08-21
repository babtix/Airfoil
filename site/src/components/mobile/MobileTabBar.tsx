import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import './MobileTabBar.css';

interface TabItem {
  name: string;
  path: string;
  icon: React.ReactNode;
}

export const MobileTabBar: React.FC = () => {
  const location = useLocation();

  const tabs: TabItem[] = [
    {
      name: 'FEED',
      path: '/feed',
      icon: (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <polygon points="12 2 2 7 12 12 22 7 12 2" />
          <polyline points="2 17 12 22 22 17" />
          <polyline points="2 12 12 17 22 12" />
        </svg>
      )
    },
    {
      name: 'DIGEST',
      path: '/digest',
      icon: (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
          <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
        </svg>
      )
    },
    {
      name: 'TOP',
      path: '/top',
      icon: (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
        </svg>
      )
    },
    {
      name: 'SHIP',
      path: '/ship',
      icon: (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
          <polyline points="22 4 12 14.01 9 11.01" />
        </svg>
      )
    },
    {
      name: 'SEARCH',
      path: '/search',
      icon: (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
      )
    }
  ];

  return (
    <nav className="mobile-tab-bar" aria-label="Mobile Navigation">
      {tabs.map(tab => {
        const isActive = location.pathname === tab.path || 
          (tab.path === '/feed' && (location.pathname === '/' || location.pathname.startsWith('/story/')));

        return (
          <Link
            key={tab.path}
            to={tab.path}
            className={`mobile-tab-item ${isActive ? 'active' : ''}`}
          >
            {isActive && <div className="mobile-tab-indicator" />}
            <div className="mobile-tab-icon">{tab.icon}</div>
            <span>{tab.name}</span>
          </Link>
        );
      })}
    </nav>
  );
};
