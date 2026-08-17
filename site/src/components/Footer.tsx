import React from 'react';

interface FooterProps {
  currentCount: number;
  totalCount: number;
  sourcesCount: number;
}

export const Footer: React.FC<FooterProps> = ({
  currentCount,
  totalCount,
  sourcesCount
}) => {
  return (
    <footer
      style={{
        borderTop: '1px solid var(--line)',
        backgroundColor: 'var(--deep)',
        marginTop: 'auto'
      }}
    >
      <div
        className="app-container"
        style={{
          padding: '0.85rem 1rem',
          display: 'flex',
          flexWrap: 'wrap',
          gap: '0.5rem 1.5rem',
          justifyContent: 'space-between',
          alignItems: 'center'
        }}
      >
        <span className="mono-label">
          {currentCount}/{totalCount} SIGNALS IN WINDOW · {sourcesCount} SOURCES TRACKED
        </span>
        <span className="mono-label" style={{ opacity: 0.8 }}>
          COMPILED BY AIRFOIL AGENT · ZERO-JS SHIP
        </span>
      </div>
    </footer>
  );
};
