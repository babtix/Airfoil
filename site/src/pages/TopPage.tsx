import React from 'react';
import type { Story, DateWindow } from '../types/story';
import { StoryCard } from '../components/StoryCard';

interface TopPageProps {
  stories: Story[];
  onOpenStory: (story: Story) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
  dateWindow: DateWindow;
  onSelectDate: (date: DateWindow) => void;
  onResetFilters?: () => void;
}

export const TopPage: React.FC<TopPageProps> = ({
  stories,
  onOpenStory,
  selectedTag,
  onSelectTag,
  dateWindow,
  onSelectDate,
  onResetFilters
}) => {
  const topStories = [...stories].sort((a, b) => b.score - a.score);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      {/* Top Header & Range Switcher */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: '0.75rem',
          borderBottom: '1px solid var(--line)',
          paddingBottom: '0.75rem'
        }}
      >
        <div>
          <h1 className="mono-label" style={{ color: 'var(--kai)', fontSize: '12px' }}>
            // LEADERBOARD — RANKED BY SIGNAL POWER
          </h1>
        </div>
        <div style={{ display: 'flex', gap: '0.35rem', alignItems: 'center' }}>
          <span className="mono-label" style={{ fontSize: '9px', marginRight: '0.25rem' }}>WINDOW:</span>
          {(['24H', '7D', 'ALL'] as const).map(tf => (
            <button
              key={tf}
              type="button"
              className={`btn-ghost ${dateWindow === tf ? 'btn-ghost-on' : ''}`}
              onClick={() => onSelectDate(tf)}
              style={{ padding: '2px 8px', fontSize: '10px' }}
            >
              {tf}
            </button>
          ))}
        </div>
      </div>

      {/* Stories list sorted by score */}
      {topStories.length > 0 ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {topStories.map((story, index) => (
            <div key={story.id} style={{ display: 'flex', gap: '0.5rem', alignItems: 'stretch' }}>
              <div
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: '12px',
                  color: index < 3 ? 'var(--kai)' : 'var(--sec)',
                  width: '28px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  flexShrink: 0,
                  borderRight: '1px solid var(--line)'
                }}
              >
                #{index + 1}
              </div>
              <div style={{ flex: 1 }}>
                <StoryCard
                  story={story}
                  onOpenStory={onOpenStory}
                  selectedTag={selectedTag}
                  onSelectTag={onSelectTag}
                />
              </div>
            </div>
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
            NO SIGNALS IN CURRENT FILTER WINDOW
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
