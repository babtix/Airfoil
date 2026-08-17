import React from 'react';
import type { FilterState } from '../types/story';

interface ActiveFilterBarProps {
  filters: FilterState;
  onClearTag: () => void;
  onClearSource: () => void;
  onClearDate: () => void;
  onClearQuery?: () => void;
  onClearSort?: () => void;
  onResetAll: () => void;
  totalFiltered: number;
  totalAvailable: number;
}

export const ActiveFilterBar: React.FC<ActiveFilterBarProps> = ({
  filters,
  onClearTag,
  onClearSource,
  onClearDate,
  onClearQuery,
  onClearSort,
  onResetAll,
  totalFiltered,
  totalAvailable
}) => {
  const hasActiveFilters =
    filters.tag !== null ||
    filters.source !== null ||
    filters.date !== 'ALL' ||
    (filters.query && filters.query.trim().length > 0) ||
    filters.sortBy !== 'newest';

  if (!hasActiveFilters) return null;

  return (
    <div
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: '0.5rem',
        padding: '0.6rem 0.85rem',
        backgroundColor: 'var(--surf)',
        border: '1px solid var(--kai-border)',
        borderRadius: '6px',
        marginBottom: '0.75rem',
        fontSize: '11px',
        fontFamily: 'var(--font-mono)'
      }}
    >
      <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '0.4rem' }}>
        <span style={{ color: 'var(--kai)', fontWeight: 600 }}>// ACTIVE:</span>

        {filters.sortBy !== 'newest' && onClearSort && (
          <button
            type="button"
            onClick={onClearSort}
            className="tagpill tagpill-on"
            style={{ fontSize: '10px', padding: '2px 6px', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
            title="Reset to newest first"
          >
            <span>SORT: {filters.sortBy.toUpperCase()}</span>
            <span style={{ color: 'var(--kai)', fontWeight: 700 }}>✕</span>
          </button>
        )}

        {filters.date !== 'ALL' && (
          <button
            type="button"
            onClick={onClearDate}
            className="tagpill tagpill-on"
            style={{ fontSize: '10px', padding: '2px 6px', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
            title="Clear date filter"
          >
            <span>WINDOW: {filters.date}</span>
            <span style={{ color: 'var(--kai)', fontWeight: 700 }}>✕</span>
          </button>
        )}

        {filters.tag && (
          <button
            type="button"
            onClick={onClearTag}
            className="tagpill tagpill-on"
            style={{ fontSize: '10px', padding: '2px 6px', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
            title="Clear tag filter"
          >
            <span>TAG: {filters.tag}</span>
            <span style={{ color: 'var(--kai)', fontWeight: 700 }}>✕</span>
          </button>
        )}

        {filters.source && (
          <button
            type="button"
            onClick={onClearSource}
            className="tagpill tagpill-on"
            style={{ fontSize: '10px', padding: '2px 6px', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
            title="Clear source filter"
          >
            <span>SRC: {filters.source}</span>
            <span style={{ color: 'var(--kai)', fontWeight: 700 }}>✕</span>
          </button>
        )}

        {filters.query && filters.query.trim().length > 0 && onClearQuery && (
          <button
            type="button"
            onClick={onClearQuery}
            className="tagpill tagpill-on"
            style={{ fontSize: '10px', padding: '2px 6px', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
            title="Clear query filter"
          >
            <span>QUERY: "{filters.query}"</span>
            <span style={{ color: 'var(--kai)', fontWeight: 700 }}>✕</span>
          </button>
        )}
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
        <span style={{ color: 'var(--sec)', fontSize: '10px' }}>
          {totalFiltered} / {totalAvailable} SIGNALS
        </span>
        <button
          type="button"
          onClick={onResetAll}
          style={{
            background: 'none',
            border: 'none',
            color: 'var(--kai)',
            textDecoration: 'underline',
            cursor: 'pointer',
            fontSize: '10px',
            fontFamily: 'var(--font-mono)',
            padding: 0
          }}
        >
          RESET ALL
        </button>
      </div>
    </div>
  );
};
