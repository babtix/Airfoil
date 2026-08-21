import React, { useRef, useEffect } from 'react';
import { Link, useLocation } from 'react-router-dom';
import type { Theme } from '../hooks/useTheme';

interface HeaderProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  isSearchOpen: boolean;
  onToggleSearch: () => void;
  onOpenMobileMenu: () => void;
  theme?: Theme;
  onToggleTheme?: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  searchQuery,
  onSearchChange,
  isSearchOpen,
  onToggleSearch,
  onOpenMobileMenu,
  theme = 'dark',
  onToggleTheme
}) => {
  const location = useLocation();
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isSearchOpen && searchInputRef.current) {
      searchInputRef.current.focus();
    }
  }, [isSearchOpen]);

  const navLinks = [
    { name: 'HOME', path: '/' },
    { name: 'FEED', path: '/feed' },
    { name: 'DIGEST', path: '/digest' },
    { name: 'TOP', path: '/top' },
    { name: 'SHIP', path: '/ship' },
    { name: 'PURGE', path: '/purge' }
  ];

  return (
    <header
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        zIndex: 50,
        backgroundColor: 'var(--header-bg)',
        backdropFilter: 'blur(10px)',
        borderBottom: '1px solid var(--line)',
        transition: 'background-color 0.2s ease, border-color 0.2s ease'
      }}
    >
      <div
        className="app-container"
        style={{
          height: 'var(--header-height)',
          display: 'flex',
          alignItems: 'center',
          gap: '1rem'
        }}
      >
        {/* Airfoil Logo & Wordmark */}
        <Link to="/" style={{ display: 'flex', alignItems: 'center', gap: '0.6rem', flexShrink: 0 }} title="Airfoil Home">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" style={{ flexShrink: 0 }}>
            <path d="M 12 3 L 3 20 L 12 16.5 L 21 20 Z" fill="var(--surf)" stroke="var(--prim)" strokeWidth="1.4" strokeLinejoin="round" />
            <line x1="6.5" y1="14" x2="17.5" y2="14" stroke="var(--kai)" strokeWidth="1.5" />
            <circle cx="12" cy="3" r="1" fill="var(--kai)" />
          </svg>
          <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, letterSpacing: '-0.02em', fontSize: '1.15rem' }}>
            <span style={{ color: 'var(--kai)' }}>AI</span>RFOIL<span style={{ color: 'var(--kai)' }}>_</span>
          </span>
        </Link>

        {/* Telemetry Subtitle */}
        <span
          className="mono-label"
          style={{
            display: 'none',
            fontSize: '10px'
          }}
          id="header-telemetry-label"
        >
          AI SIGNAL TELEMETRY // {theme === 'dark' ? 'KAIOKEN' : 'CLAIRE'}
        </span>

        {/* Navigation Tabs */}
        <nav style={{ display: 'none', alignItems: 'center', gap: '0.4rem', marginLeft: '1rem' }} id="header-nav">
          {navLinks.map(link => {
            const isActive = location.pathname === link.path;
            return (
              <Link
                key={link.path}
                to={link.path}
                className={`btn-ghost ${isActive ? 'btn-ghost-on' : ''}`}
                style={{ fontSize: '11px', padding: '3px 8px' }}
              >
                {link.name}
              </Link>
            );
          })}
        </nav>

        {/* Right Action Surfaces */}
        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '0.45rem' }}>
          {/* Search Toggle */}
          <button
            type="button"
            onClick={onToggleSearch}
            className={`btn-ghost ${isSearchOpen || searchQuery ? 'btn-ghost-on' : ''}`}
            title="Toggle Search (Cmd/Ctrl + K)"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <circle cx="10" cy="10" r="7" />
              <line x1="15" y1="15" x2="22" y2="22" />
            </svg>
            <span style={{ display: 'inline' }}>SEARCH</span>
          </button>

          {/* Light / Dark Mode Toggle */}
          {onToggleTheme && (
            <button
              type="button"
              onClick={onToggleTheme}
              className="btn-ghost"
              title={theme === 'dark' ? 'Switch to White / Claire Mode' : 'Switch to Dark / Kaioken Mode'}
              style={{ padding: '0.25rem 0.5rem' }}
            >
              {theme === 'dark' ? (
                /* Sun Icon */
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="5" />
                  <line x1="12" y1="1" x2="12" y2="3" />
                  <line x1="12" y1="21" x2="12" y2="23" />
                  <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
                  <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
                  <line x1="1" y1="12" x2="3" y2="12" />
                  <line x1="21" y1="12" x2="23" y2="12" />
                  <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
                  <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
                </svg>
              ) : (
                /* Moon Icon */
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
                </svg>
              )}
              <span className="sm-inline">{theme === 'dark' ? 'LIGHT' : 'DARK'}</span>
            </button>
          )}

          {/* GitHub Link */}
          <a
            href="https://github.com"
            target="_blank"
            rel="noopener noreferrer"
            className="btn-ghost"
            title="GitHub Repository"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.11.79-.25.79-.56v-2c-3.2.7-3.87-1.54-3.87-1.54-.52-1.33-1.28-1.68-1.28-1.68-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.03 1.76 2.7 1.25 3.35.96.1-.75.4-1.25.72-1.54-2.55-.29-5.24-1.28-5.24-5.68 0-1.26.45-2.28 1.18-3.09-.12-.29-.51-1.46.11-3.05 0 0 .96-.31 3.15 1.18a10.9 10.9 0 0 1 5.74 0c2.19-1.49 3.15-1.18 3.15-1.18.62 1.59.23 2.76.11 3.05.73.81 1.18 1.83 1.18 3.09 0 4.41-2.69 5.38-5.26 5.66.41.36.78 1.06.78 2.14v3.17c0 .31.21.68.8.56A11.5 11.5 0 0 0 23.5 12C23.5 5.65 18.35.5 12 .5Z" />
            </svg>
            <span style={{ display: 'none' }} className="sm-inline">GITHUB</span>
          </a>

          {/* Mobile Menu Trigger */}
          <button
            type="button"
            onClick={onOpenMobileMenu}
            className="btn-ghost"
            style={{ display: 'inline-flex' }}
            id="mobile-menu-btn"
            title="Open Control Surfaces"
          >
            <svg width="14" height="12" viewBox="0 0 14 12" stroke="currentColor" strokeWidth="2">
              <line x1="0" y1="1" x2="14" y2="1" />
              <line x1="0" y1="6" x2="14" y2="6" />
              <line x1="0" y1="11" x2="14" y2="11" />
            </svg>
          </button>
        </div>
      </div>

      {/* Expandable Global Search Row */}
      {isSearchOpen && (
        <div
          style={{
            borderTop: '1px solid var(--line)',
            backgroundColor: 'var(--deep)',
            padding: '0.5rem 0'
          }}
        >
          <div className="app-container" style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
            <input
              ref={searchInputRef}
              type="text"
              value={searchQuery}
              onChange={e => onSearchChange(e.target.value)}
              placeholder="> query signals…"
              style={{
                width: '100%',
                backgroundColor: 'var(--surf)',
                border: '1px solid var(--line)',
                borderRadius: '4px',
                padding: '0.5rem 0.75rem',
                fontFamily: 'var(--font-mono)',
                fontSize: '0.875rem',
                color: 'var(--prim)'
              }}
            />
            {searchQuery && (
              <button
                type="button"
                onClick={() => onSearchChange('')}
                className="btn-ghost"
                style={{ flexShrink: 0, padding: '0.4rem 0.6rem' }}
                title="Clear query"
              >
                ✕
              </button>
            )}
          </div>
        </div>
      )}

      {/* Responsive Inline CSS for Header */}
      <style>{`
        @media (min-width: 640px) {
          .sm-inline { display: inline !important; }
        }
        @media (min-width: 768px) {
          #header-telemetry-label { display: block !important; }
          #header-nav { display: flex !important; }
        }
        @media (min-width: 1024px) {
          #mobile-menu-btn { display: none !important; }
        }
      `}</style>
    </header>
  );
};
