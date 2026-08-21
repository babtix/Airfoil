import React, { useEffect } from 'react';
import type { DateWindow, SortOrder } from '../../types/story';

interface MobileFilterDrawerProps {
  isOpen: boolean;
  onClose: () => void;
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
  onResetFilters: () => void;
}

export const MobileFilterDrawer: React.FC<MobileFilterDrawerProps> = ({
  isOpen,
  onClose,
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
  // Prevent body scroll when drawer is open
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return () => {
      document.body.style.overflow = '';
    };
  }, [isOpen]);

  if (!isOpen) return null;

  const dateOptions: DateWindow[] = ['ALL', '24H', '7D'];
  const sortOptions: { id: SortOrder; label: string }[] = [
    { id: 'newest', label: 'NEWEST' },
    { id: 'score', label: 'TOP SIGNAL' },
    { id: 'cluster', label: 'HOT CLUSTER' }
  ];

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 100,
        backgroundColor: 'rgba(0, 0, 0, 0.75)',
        backdropFilter: 'blur(8px)',
        WebkitBackdropFilter: 'blur(8px)',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'flex-end'
      }}
      onClick={onClose}
    >
      <div
        className="mobile-drawer-content"
        style={{
          backgroundColor: 'var(--deep)',
          borderTop: '1px solid var(--line)',
          borderRadius: '16px 16px 0 0',
          maxHeight: '85vh',
          overflowY: 'auto',
          padding: '1.25rem 1.25rem calc(var(--safe-bottom) + 2rem)',
          display: 'flex',
          flexDirection: 'column',
          gap: '1.5rem',
          boxShadow: '0 -8px 30px rgba(0, 0, 0, 0.8)'
        }}
        onClick={e => e.stopPropagation()}
      >
        {/* Drag handle & Header */}
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.75rem' }}>
          <div
            style={{
              width: '36px',
              height: '4px',
              backgroundColor: 'var(--line-light)',
              borderRadius: '2px'
            }}
          />
          <div
            style={{
              width: '100%',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              borderBottom: '1px solid var(--line)',
              paddingBottom: '0.65rem'
            }}
          >
            <span className="mono-label" style={{ color: 'var(--kai)', fontWeight: 600 }}>
              // TELEMETRY FILTERS
            </span>
            <button
              type="button"
              onClick={onClose}
              className="btn-ghost"
              style={{ padding: '0.25rem 0.5rem', fontSize: '10px' }}
            >
              DONE
            </button>
          </div>
        </div>

        {/* Sort Order */}
        {onSelectSortBy && (
          <div>
            <div className="rail-h">// ORDER BY</div>
            <div style={{ display: 'flex', gap: '0.4rem' }}>
              {sortOptions.map(s => (
                <button
                  key={s.id}
                  type="button"
                  onClick={() => onSelectSortBy(s.id)}
                  className={`btn-ghost ${sortBy === s.id ? 'btn-ghost-on' : ''}`}
                  style={{ flex: 1, padding: '0.45rem 0.2rem', fontSize: '10px' }}
                >
                  {s.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Date Window */}
        <div>
          <div className="rail-h">// DATE WINDOW</div>
          <div style={{ display: 'flex', gap: '0.4rem' }}>
            {dateOptions.map(d => (
              <button
                key={d}
                type="button"
                onClick={() => onSelectDate(d)}
                className={`btn-ghost ${dateWindow === d ? 'btn-ghost-on' : ''}`}
                style={{ flex: 1, padding: '0.45rem 0.2rem', fontSize: '11px' }}
              >
                {d}
              </button>
            ))}
          </div>
        </div>

        {/* Tags */}
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }} className="rail-h">
            <span>// TOP TAGS</span>
            {selectedTag && (
              <button
                type="button"
                onClick={() => onSelectTag(selectedTag)}
                style={{ color: 'var(--kai)', fontSize: '9px', textDecoration: 'underline' }}
              >
                CLEAR
              </button>
            )}
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.4rem' }}>
            {tags.map(t => (
              <button
                key={t}
                type="button"
                onClick={() => onSelectTag(t)}
                className={`mobile-tagpill ${selectedTag === t ? 'mobile-tagpill-active' : ''}`}
              >
                {t}
              </button>
            ))}
          </div>
        </div>

        {/* Sources */}
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }} className="rail-h">
            <span>// TOP SOURCES</span>
            {selectedSource && (
              <button
                type="button"
                onClick={() => onSelectSource(selectedSource)}
                style={{ color: 'var(--kai)', fontSize: '9px', textDecoration: 'underline' }}
              >
                CLEAR
              </button>
            )}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.35rem' }}>
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
                    padding: '0.45rem 0.5rem',
                    borderRadius: '4px',
                    backgroundColor: isSelected ? 'rgba(255, 59, 59, 0.1)' : 'transparent',
                    border: isSelected ? '1px solid var(--kai-border)' : '1px solid transparent',
                    color: isSelected ? 'var(--kai)' : 'var(--prim)'
                  }}
                >
                  <span>{name}</span>
                  <span style={{ fontSize: '10px', opacity: 0.7 }}>×{count}</span>
                </button>
              );
            })}
          </div>
        </div>

        {/* Reset All Button */}
        <button
          type="button"
          onClick={() => {
            onResetFilters();
            onClose();
          }}
          className="btn-ghost"
          style={{
            width: '100%',
            borderColor: 'var(--kai-dim)',
            color: 'var(--kai)',
            padding: '0.65rem 0',
            marginTop: '0.5rem',
            fontWeight: 600
          }}
        >
          RESET ALL FILTERS
        </button>
      </div>
    </div>
  );
};
