import React from 'react';
import type { Story, FilterState, SortOrder } from '../types/story';
import { StoryCard } from '../components/StoryCard';
import { ActiveFilterBar } from '../components/ActiveFilterBar';

interface ShipPageProps {
  stories: Story[];
  totalStoriesCount: number;
  filters: FilterState;
  onClearTag: () => void;
  onClearSource: () => void;
  onClearDate: () => void;
  onClearQuery: () => void;
  onSelectSortBy?: (sort: SortOrder) => void;
  onOpenStory: (story: Story) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
  onResetFilters?: () => void;
}

export const ShipPage: React.FC<ShipPageProps> = ({
  stories,
  totalStoriesCount,
  filters,
  onClearTag,
  onClearSource,
  onClearDate,
  onClearQuery,
  onSelectSortBy,
  onOpenStory,
  selectedTag,
  onSelectTag,
  onResetFilters
}) => {
  const builderStories = stories.filter(s => s.builder_relevant !== false);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      {/* Ship Header Banner */}
      <div
        style={{
          border: '1px solid var(--line)',
          borderRadius: '6px',
          backgroundColor: 'var(--surf)',
          padding: '1rem 1.25rem',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: '1rem'
        }}
      >
        <div>
          <span className="mono-label" style={{ color: 'var(--kai)', display: 'block', marginBottom: '0.25rem' }}>
            // BUILDER CUT — SHIP SIGNALS ONLY
          </span>
          <p style={{ fontSize: '0.85rem', color: 'var(--sec)' }}>
            Model releases, open weights, SDK changelogs, APIs, and benchmarks with reproducible code.
          </p>
        </div>
        <span
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: '11px',
            padding: '4px 8px',
            borderRadius: '4px',
            border: '1px solid var(--kai-border)',
            color: 'var(--kai)',
            backgroundColor: 'rgba(255, 59, 59, 0.08)',
            flexShrink: 0
          }}
        >
          {builderStories.length} RELEASES
        </span>
      </div>

      {/* Sort By controls for Ship cut */}
      {onSelectSortBy && (
        <div
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            alignItems: 'center',
            gap: '0.35rem',
            paddingBottom: '0.25rem'
          }}
        >
          <span className="mono-label" style={{ fontSize: '9px', marginRight: '0.2rem' }}>ORDER BY:</span>
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
            >
              {s.label}
            </button>
          ))}
        </div>
      )}

      <ActiveFilterBar
        filters={filters}
        onClearTag={onClearTag}
        onClearSource={onClearSource}
        onClearDate={onClearDate}
        onClearQuery={onClearQuery}
        onResetAll={onResetFilters || (() => {})}
        totalFiltered={builderStories.length}
        totalAvailable={totalStoriesCount}
      />

      {/* Stories list */}
      {builderStories.length > 0 ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {builderStories.map(story => (
            <StoryCard
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
            borderRadius: '6px',
            padding: '3rem 1.5rem',
            textAlign: 'center',
            backgroundColor: 'var(--surf)'
          }}
        >
          <p className="mono-label" style={{ marginBottom: '1rem', color: 'var(--sec)' }}>
            NO SHIP SIGNALS IN CURRENT FILTER WINDOW
          </p>
          {onResetFilters && (
            <button
              type="button"
              onClick={onResetFilters}
              className="btn-ghost btn-ghost-on"
            >
              RESET ALL FILTERS
            </button>
          )}
        </div>
      )}
    </div>
  );
};
