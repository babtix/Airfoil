import React from 'react';
import type { Story, FilterState, SortOrder } from '../types/story';
import { StoryCard } from '../components/StoryCard';
import { ActiveFilterBar } from '../components/ActiveFilterBar';

interface FeedPageProps {
  stories: Story[];
  totalStoriesCount: number;
  filters: FilterState;
  onClearTag: () => void;
  onClearSource: () => void;
  onClearDate: () => void;
  onClearQuery: () => void;
  onSelectSortBy?: (sort: SortOrder) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
  onOpenStory: (story: Story) => void;
  onResetFilters: () => void;
}

export const FeedPage: React.FC<FeedPageProps> = ({
  stories,
  totalStoriesCount,
  filters,
  onClearTag,
  onClearSource,
  onClearDate,
  onClearQuery,
  onSelectSortBy,
  selectedTag,
  onSelectTag,
  onOpenStory,
  onResetFilters
}) => {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
      {/* Stream Controls Header */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: '0.5rem',
          borderBottom: '1px solid var(--line)',
          paddingBottom: '0.5rem'
        }}
      >
        <div className="mono-label" style={{ color: 'var(--kai)', fontSize: '11px' }}>
          // LIVE SIGNAL STREAM ({stories.length} SIGNALS)
        </div>

        {onSelectSortBy && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
            <span className="mono-label" style={{ fontSize: '9px', marginRight: '0.2rem' }}>SORT:</span>
            {[
              { id: 'newest', label: 'NEWEST' },
              { id: 'score', label: 'TOP SIGNAL' },
              { id: 'cluster', label: 'HOT CLUSTER' }
            ].map(s => (
              <button
                key={s.id}
                type="button"
                className={`btn-ghost ${filters.sortBy === s.id ? 'btn-ghost-on' : ''}`}
                onClick={() => onSelectSortBy(s.id as SortOrder)}
                style={{ padding: '2px 8px', fontSize: '9px' }}
                title={`Sort by ${s.label.toLowerCase()}`}
              >
                {s.label}
              </button>
            ))}
          </div>
        )}
      </div>

      <ActiveFilterBar
        filters={filters}
        onClearTag={onClearTag}
        onClearSource={onClearSource}
        onClearDate={onClearDate}
        onClearQuery={onClearQuery}
        onResetAll={onResetFilters}
        totalFiltered={stories.length}
        totalAvailable={totalStoriesCount}
      />

      {stories.length > 0 ? (
        stories.map(story => (
          <StoryCard
            key={story.id}
            story={story}
            onOpenStory={onOpenStory}
            selectedTag={selectedTag}
            onSelectTag={onSelectTag}
          />
        ))
      ) : (
        <div
          style={{
            border: '1px solid var(--line)',
            borderRadius: '6px',
            padding: '3rem 1.5rem',
            textAlign: 'center',
            backgroundColor: 'var(--surf)'
          }}
        >
          <p className="mono-label" style={{ marginBottom: '1rem', color: 'var(--sec)' }}>
            NO SIGNALS IN WINDOW — ADJUST FILTERS
          </p>
          <button
            type="button"
            onClick={onResetFilters}
            className="btn-ghost btn-ghost-on"
          >
            RESET ALL FILTERS
          </button>
        </div>
      )}
    </div>
  );
};
