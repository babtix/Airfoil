import React from 'react';
import type { Story, FilterState, SortOrder, DateWindow } from '../../types/story';
import { MobileStoryCard } from '../../components/mobile/MobileStoryCard';

interface MobileFeedPageProps {
  stories: Story[];
  totalStoriesCount: number;
  filters: FilterState;
  onClearTag: () => void;
  onClearSource: () => void;
  onClearDate: () => void;
  onClearQuery: () => void;
  onSelectDate: (date: DateWindow) => void;
  onSelectSortBy?: (sort: SortOrder) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
  onOpenStory: (story: Story) => void;
  onResetFilters: () => void;
  topStory: Story | null;
}

export const MobileFeedPage: React.FC<MobileFeedPageProps> = ({
  stories,
  totalStoriesCount,
  filters,
  onClearTag,
  onClearSource,
  onClearDate,
  onSelectDate,
  onSelectSortBy,
  selectedTag,
  onSelectTag,
  onOpenStory,
  onResetFilters,
  topStory
}) => {
  const hasActiveFilters =
    filters.tag !== null ||
    filters.source !== null ||
    filters.date !== 'ALL' ||
    Boolean(filters.query && filters.query.trim().length > 0) ||
    filters.sortBy !== 'newest';

  const dateOptions: DateWindow[] = ['ALL', '24H', '7D'];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
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

        {/* Active Date Chip */}
        {filters.date !== 'ALL' && (
          <button
            type="button"
            onClick={onClearDate}
            className="tagpill tagpill-on"
            style={{ padding: '4px 8px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
          >
            <span>DATE: {filters.date}</span>
            <span style={{ fontWeight: 700 }}>✕</span>
          </button>
        )}
      </div>

      {/* Mach Leader Top Signal Highlight if on default view without heavy filtering */}
      {!hasActiveFilters && topStory && (
        <div
          className="mobile-card"
          onClick={() => onOpenStory(topStory)}
          style={{
            borderColor: 'var(--kai-border)',
            backgroundColor: 'rgba(255, 59, 59, 0.04)',
            padding: '1rem',
            marginBottom: '0.25rem'
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.4rem' }}>
            <span className="mono-label" style={{ color: 'var(--kai)', fontSize: '10px' }}>
              ★ TODAY'S MACH LEADER
            </span>
            <span
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                color: 'var(--kai)',
                fontWeight: 700,
                border: '1px solid var(--kai-border)',
                borderRadius: '4px',
                padding: '1px 6px'
              }}
            >
              {topStory.score}
            </span>
          </div>
          <h3 style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--prim)', margin: '0 0 0.35rem 0', lineHeight: 1.35 }}>
            {topStory.title}
          </h3>
          <p className="line-clamp-2" style={{ fontSize: '0.825rem', color: 'var(--sec)', margin: 0, lineHeight: 1.45 }}>
            {topStory.summary}
          </p>
        </div>
      )}

      {/* Telemetry Stream Counter Banner */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          borderBottom: '1px solid var(--line)',
          paddingBottom: '0.4rem',
          paddingTop: '0.1rem'
        }}
      >
        <span className="mono-label" style={{ color: 'var(--kai)', fontSize: '10px' }}>
          // LIVE TELEMETRY
        </span>
        <span className="mono-label" style={{ fontSize: '10px', color: 'var(--sec)' }}>
          {stories.length} OF {totalStoriesCount} SIGNALS
        </span>
      </div>

      {/* Story List */}
      {stories.length > 0 ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {stories.map(story => (
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
            NO SIGNALS IN WINDOW — ADJUST FILTERS
          </p>
          <button
            type="button"
            onClick={onResetFilters}
            className="btn-ghost btn-ghost-on"
            style={{ padding: '0.5rem 1rem', fontSize: '11px' }}
          >
            RESET ALL FILTERS
          </button>
        </div>
      )}
    </div>
  );
};
