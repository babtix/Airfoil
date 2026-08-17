import React from 'react';
import type { DateWindow, SortOrder } from '../types/story';
import { TagPill } from './TagPill';

interface LeftRailProps {
  dateWindow: DateWindow;
  onSelectDate: (date: DateWindow) => void;
  sortBy?: SortOrder;
  onSelectSortBy?: (sort: SortOrder) => void;
  tags: string[];
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
  topSources: [string, number][];
  selectedSource: string | null;
  onSelectSource: (source: string) => void;
  onResetFilters?: () => void;
}

export const LeftRail: React.FC<LeftRailProps> = ({
  dateWindow,
  onSelectDate,
  sortBy = 'newest',
  onSelectSortBy,
  tags,
  selectedTag,
  onSelectTag,
  topSources,
  selectedSource,
  onSelectSource,
  onResetFilters
}) => {
  const dateOptions: DateWindow[] = ['ALL', '24H', '7D'];
  const sortOptions: { id: SortOrder; label: string }[] = [
    { id: 'newest', label: 'NEWEST' },
    { id: 'score', label: 'SCORE' },
    { id: 'cluster', label: 'HOT' }
  ];
  const hasActiveFilter = dateWindow !== 'ALL' || selectedTag !== null || selectedSource !== null || sortBy !== 'newest';

  return (
    <aside style={{ width: '100%' }}>
      {/* Sort By / Order By Filter */}
      {onSelectSortBy && (
        <>
          <div className="rail-h">// ORDER BY</div>
          <div style={{ display: 'flex', gap: '0.35rem', marginBottom: '1.5rem' }}>
            {sortOptions.map(s => (
              <button
                key={s.id}
                type="button"
                className={`btn-ghost ${sortBy === s.id ? 'btn-ghost-on' : ''}`}
                onClick={() => onSelectSortBy(s.id)}
                style={{ flex: 1, fontSize: '10px', padding: '4px 2px' }}
                title={`Sort by ${s.label.toLowerCase()}`}
              >
                {s.label}
              </button>
            ))}
          </div>
        </>
      )}

      {/* Date Window */}
      <div className="rail-h">// DATE WINDOW</div>
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1.5rem' }}>
        {dateOptions.map(d => (
          <button
            key={d}
            type="button"
            className={`btn-ghost ${dateWindow === d ? 'btn-ghost-on' : ''}`}
            onClick={() => onSelectDate(d)}
            style={{ flex: 1 }}
          >
            {d}
          </button>
        ))}
      </div>

      {/* Tags Filter */}
      <div className="rail-h" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span>// TAGS</span>
        {selectedTag && (
          <button
            type="button"
            onClick={() => onSelectTag(selectedTag)}
            style={{ fontFamily: 'var(--font-mono)', fontSize: '9px', color: 'var(--kai)', textDecoration: 'underline' }}
          >
            CLEAR
          </button>
        )}
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.375rem', marginBottom: '1.5rem' }}>
        {tags.map(t => (
          <TagPill
            key={t}
            tag={t}
            isActive={selectedTag === t}
            onClick={() => onSelectTag(t)}
          />
        ))}
      </div>

      {/* Top Sources Filter */}
      <div className="rail-h" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span>// TOP SOURCES</span>
        {selectedSource && (
          <button
            type="button"
            onClick={() => onSelectSource(selectedSource)}
            style={{ fontFamily: 'var(--font-mono)', fontSize: '9px', color: 'var(--kai)', textDecoration: 'underline' }}
          >
            CLEAR
          </button>
        )}
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.375rem', marginBottom: '1.5rem' }}>
        {topSources.map(([name, count]) => {
          const isSelected = selectedSource === name;
          return (
            <button
              key={name}
              type="button"
              onClick={() => onSelectSource(name)}
              style={{
                width: '100%',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                fontFamily: 'var(--font-mono)',
                fontSize: '12px',
                padding: '0.25rem 0',
                borderBottom: '1px solid rgba(38, 38, 38, 0.6)',
                color: isSelected ? 'var(--kai)' : 'var(--sec)',
                transition: 'color 0.15s ease'
              }}
              className="source-row"
            >
              <span>{name}</span>
              <span style={{ fontSize: '10px', opacity: 0.8 }}>×{count}</span>
            </button>
          );
        })}
      </div>

      {/* Reset Filter Button */}
      {hasActiveFilter && onResetFilters && (
        <button
          type="button"
          onClick={onResetFilters}
          className="btn-ghost"
          style={{ width: '100%', borderColor: 'var(--kai-dim)', color: 'var(--kai)', marginTop: '0.5rem' }}
        >
          RESET ALL FILTERS
        </button>
      )}

      <style>{`
        .source-row:hover {
          color: var(--prim) !important;
        }
      `}</style>
    </aside>
  );
};
