import React from 'react';

interface ScoreBadgeProps {
  score: number;
  className?: string;
}

export const ScoreBadge: React.FC<ScoreBadgeProps> = ({ score, className = '' }) => {
  const isHigh = score > 80;
  return (
    <span
      className={`font-mono text-sm font-medium px-2 py-0.5 rounded border inline-flex items-center justify-center shrink-0 ${
        isHigh
          ? 'text-[var(--kai)] border-[var(--kai-border)] bg-[rgba(255,59,59,0.06)]'
          : 'text-[var(--sec)] border-[var(--line)] bg-[var(--surf)]'
      } ${className}`}
      style={{
        fontFamily: 'var(--font-mono)',
        fontSize: '13px',
        fontWeight: 500,
        padding: '2px 8px',
        borderRadius: '4px',
        borderWidth: '1px',
        borderStyle: 'solid',
        borderColor: isHigh ? 'var(--kai-border)' : 'var(--line)',
        color: isHigh ? 'var(--kai)' : 'var(--sec)',
        backgroundColor: isHigh ? 'rgba(255, 59, 59, 0.06)' : 'var(--surf)'
      }}
    >
      {score}
    </span>
  );
};
