import React from 'react';
import type { Story, FilterState, SortOrder, DateWindow } from '../../types/story';
import { MobileStoryCard } from '../../components/mobile/MobileStoryCard';

interface MobileShipPageProps {
  stories: Story[];
  totalStoriesCount: number;
  filters: FilterState;
  onClearTag: () => void;
  onClearSource: () => void;
  onClearDate: () => void;
  onClearQuery: () => void;
  onSelectDate: (date: DateWindow) => void;
  onSelectSortBy?: (sort: SortOrder) => void;
  onOpenStory: (story: Story) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
  onResetFilters?: () => void;
}

export const MobileShipPage: React.FC<MobileShipPageProps> = ({
  stories,
  totalStoriesCount,
  filters,
  onClearTag,
  onClearSource,
  onSelectDate,
  onSelectSortBy,
  onOpenStory,
  selectedTag,
  onSelectTag,
  onResetFilters
}) => {
  const builderStories = stories.filter(s => s.builder_relevant !== false);
  const dateOptions: DateWindow[] = ['ALL', '24H', '7D'];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
      {/* Ship Banner */}
      <div
        className="mobile-card"
        style={{
          cursor: 'default',
          borderLeft: '3px solid var(--kai)',
          padding: '0.85rem 1rem',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: '0.5rem'
        }}
      >
        <div>
          <span className="mono-label" style={{ color: 'var(--kai)', fontSize: '10px', display: 'block', marginBottom: '0.2rem' }}>
            // BUILDER CUT — SHIP SIGNALS
          </span>
          <p style={{ fontSize: '0.78rem', color: 'var(--sec)', margin: 0, lineHeight: 1.4 }}>
            Open weights, SDK changelogs, APIs, and benchmarks.
          </p>
        </div>
        <span
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: '10px',
            padding: '3px 6px',
            borderRadius: '4px',
            border: '1px solid var(--kai-border)',
            color: 'var(--kai)',
            backgroundColor: 'rgba(255, 59, 59, 0.08)',
            fontWeight: 700,
            whiteSpace: 'nowrap'
          }}
        >
          {builderStories.length} / {totalStoriesCount} RELEASES
        </span>
      </div>

      {/* Quick Filter Scroller Bar */}
      <div className="mobile-pill-scroller">
        {/* Date Window Buttons */}
        {dateOptions.map(d => (
          <button
            key={d}
            type="button"
            onClick={() => onSelectDate(d)}
            className={`tagpill ${filters.date === d ? 'tagpill-on' : ''}`}
            style={{ padding: '4px 8px', fontSize: '11px' }}
          >
            {d}
          </button>
        ))}

        {/* Sort Pill Switcher */}
        {onSelectSortBy && (
          <button
            type="button"
            onClick={() => {
              const nextSort: SortOrder =
                filters.sortBy === 'newest'
                  ? 'score'
                  : filters.sortBy === 'score'
                  ? 'cluster'
                  : 'newest';
              onSelectSortBy(nextSort);
            }}
            className={`tagpill ${filters.sortBy !== 'newest' ? 'tagpill-on' : ''}`}
            style={{ padding: '4px 8px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '3px' }}
          >
            <span>SORT:</span>
            <span style={{ fontWeight: 600 }}>{filters.sortBy.toUpperCase()}</span>
          </button>
        )}

        {/* Active Tag Chip */}
        {filters.tag && (
          <button
            type="button"
            onClick={onClearTag}
            className="tagpill tagpill-on"
            style={{ padding: '4px 8px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
          >
            <span>#{filters.tag}</span>
            <span style={{ fontWeight: 700 }}>✕</span>
          </button>
        )}

        {/* Active Source Chip */}
        {filters.source && (
          <button
            type="button"
            onClick={onClearSource}
            className="tagpill tagpill-on"
            style={{ padding: '4px 8px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
          >
            <span>SRC: {filters.source}</span>
            <span style={{ fontWeight: 700 }}>✕</span>
          </button>
        )}
      </div>

      {/* Story List */}
      {builderStories.length > 0 ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {builderStories.map(story => (
            <MobileStoryCard
              key={story.id}
              story={story}
              onOpenStory={onOpenStory}
              selectedTag={selectedTag}
              onSelectTag={onSelectTag}
            />
          ))}
        </div>
      ) : (
        <div
          style={{
            border: '1px solid var(--line)',
            borderRadius: '8px',
            padding: '2.5rem 1rem',
            textAlign: 'center',
            backgroundColor: 'var(--surf)',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: '0.75rem'
          }}
        >
          <p className="mono-label" style={{ color: 'var(--sec)', fontSize: '11px' }}>
            NO SHIP SIGNALS IN CURRENT WINDOW
          </p>
          {onResetFilters && (
            <button
              type="button"
              onClick={onResetFilters}
              className="btn-ghost btn-ghost-on"
              style={{ padding: '0.5rem 1rem', fontSize: '11px' }}
            >
              RESET ALL FILTERS
            </button>
          )}
        </div>
      )}
    </div>
  );
};
