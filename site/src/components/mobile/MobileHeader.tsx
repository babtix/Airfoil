import React from 'react';
import { Link } from 'react-router-dom';
import type { Theme } from '../../hooks/useTheme';

interface MobileHeaderProps {
  theme?: Theme;
  onToggleTheme?: () => void;
  onOpenFilters: () => void;
  activeFilterCount: number;
}

export const MobileHeader: React.FC<MobileHeaderProps> = ({
  theme = 'dark',
  onToggleTheme,
  onOpenFilters,
  activeFilterCount
}) => {
  return (
    <header
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        zIndex: 50,
        height: 'calc(var(--header-height) + var(--safe-top))',
        paddingTop: 'var(--safe-top)',
        backgroundColor: 'var(--header-bg)',
        backdropFilter: 'blur(16px)',
        WebkitBackdropFilter: 'blur(16px)',
        borderBottom: '1px solid var(--line)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        paddingLeft: 'var(--mobile-pad)',
        paddingRight: 'var(--mobile-pad)',
        transition: 'background-color 0.2s ease, border-color 0.2s ease'
      }}
    >
      {/* Brand logo & wordmark */}
      <Link
        to="/feed"
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '0.45rem',
          textDecoration: 'none',
          color: 'inherit'
        }}
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
          <path d="M 12 3 L 3 20 L 12 16.5 L 21 20 Z" fill="var(--surf)" stroke="var(--prim)" strokeWidth="1.4" strokeLinejoin="round" />
          <line x1="6.5" y1="14" x2="17.5" y2="14" stroke="var(--kai)" strokeWidth="1.5" />
          <circle cx="12" cy="3" r="1" fill="var(--kai)" />
        </svg>
        <span
          style={{
            fontFamily: 'var(--font-mono)',
            fontWeight: 700,
            letterSpacing: '-0.02em',
            fontSize: '1.05rem'
          }}
        >
          <span style={{ color: 'var(--kai)' }}>AI</span>RFOIL<span style={{ color: 'var(--kai)' }}>_</span>
        </span>
      </Link>

      {/* Right controls */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
        {/* Filter Trigger Button with Active Count Badge */}
        <button
          type="button"
          onClick={onOpenFilters}
          className="btn-ghost"
          style={{
            padding: '0.3rem 0.55rem',
            fontSize: '10px',
            display: 'flex',
            alignItems: 'center',
            gap: '0.35rem',
            borderColor: activeFilterCount > 0 ? 'var(--kai-border)' : 'var(--line)',
            color: activeFilterCount > 0 ? 'var(--kai)' : 'var(--sec)',
            backgroundColor: activeFilterCount > 0 ? 'rgba(255, 59, 59, 0.1)' : 'var(--surf)'
          }}
          title="Open Filters"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
            <line x1="4" y1="21" x2="4" y2="14" />
            <line x1="4" y1="10" x2="4" y2="3" />
            <line x1="12" y1="21" x2="12" y2="12" />
            <line x1="12" y1="8" x2="12" y2="3" />
            <line x1="20" y1="21" x2="20" y2="16" />
            <line x1="20" y1="12" x2="20" y2="3" />
            <line x1="1" y1="14" x2="7" y2="14" />
            <line x1="9" y1="8" x2="15" y2="8" />
            <line x1="17" y1="16" x2="23" y2="16" />
          </svg>
          <span>FILTER</span>
          {activeFilterCount > 0 && (
            <span
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                backgroundColor: 'var(--kai)',
                color: 'var(--deep)',
                borderRadius: '50%',
                width: '14px',
                height: '14px',
                fontSize: '9px',
                fontWeight: 700
              }}
            >
              {activeFilterCount}
            </span>
          )}
        </button>

        {/* Theme toggle */}
        {onToggleTheme && (
          <button
            type="button"
            onClick={onToggleTheme}
            className="btn-ghost"
            style={{ padding: '0.3rem 0.5rem' }}
            title={theme === 'dark' ? 'Light Mode' : 'Dark Mode'}
          >
            {theme === 'dark' ? (
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
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
              </svg>
            )}
          </button>
        )}
      </div>
    </header>
  );
};
