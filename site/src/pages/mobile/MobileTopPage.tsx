import React from 'react';
import type { Story, DateWindow } from '../../types/story';
import { MobileStoryCard } from '../../components/mobile/MobileStoryCard';

interface MobileTopPageProps {
  stories: Story[];
  onOpenStory: (story: Story) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
  dateWindow: DateWindow;
  onSelectDate: (date: DateWindow) => void;
  onResetFilters?: () => void;
}

export const MobileTopPage: React.FC<MobileTopPageProps> = ({
  stories,
  onOpenStory,
  selectedTag,
  onSelectTag,
  dateWindow,
  onSelectDate,
  onResetFilters
}) => {
  const topStories = [...stories].sort((a, b) => b.score - a.score);
  const timeframes: DateWindow[] = ['24H', '7D', 'ALL'];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
      {/* Top Header with time window toggle */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          borderBottom: '1px solid var(--line)',
          paddingBottom: '0.5rem'
        }}
      >
        <span className="mono-label" style={{ color: 'var(--kai)', fontSize: '11px' }}>
          // LEADERBOARD
        </span>

        <div style={{ display: 'flex', gap: '0.3rem' }}>
          {timeframes.map(tf => (
            <button
              key={tf}
              type="button"
              onClick={() => onSelectDate(tf)}
              className={`tagpill ${dateWindow === tf ? 'tagpill-on' : ''}`}
              style={{ padding: '3px 8px', fontSize: '10px' }}
            >
              {tf}
            </button>
          ))}
        </div>
      </div>

      {/* Stories list with ranking */}
      {topStories.length > 0 ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {topStories.map((story, index) => (
            <div key={story.id} style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem', paddingLeft: '0.2rem' }}>
                <span
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: '11px',
                    fontWeight: 700,
                    color: index < 3 ? 'var(--kai)' : 'var(--sec)'
                  }}
                >
                  #{index + 1} // SCORE {story.score}
                </span>
              </div>
              <MobileStoryCard
                story={story}
                onOpenStory={onOpenStory}
                selectedTag={selectedTag}
                onSelectTag={onSelectTag}
              />
            </div>
          ))}
        </div>
      ) : (
        <div
          style={{
            border: '1px solid var(--line)',
            borderRadius: '8px',
            padding: '2.5rem 1rem',
            textAlign: 'center',
            backgroundColor: 'var(--surf)'
          }}
        >
          <p className="mono-label" style={{ color: 'var(--sec)', fontSize: '11px', marginBottom: '0.75rem' }}>
            NO SIGNALS IN WINDOW
          </p>
          {onResetFilters && (
            <button
              type="button"
              onClick={onResetFilters}
              className="btn-ghost btn-ghost-on"
              style={{ padding: '0.4rem 0.85rem', fontSize: '11px' }}
            >
              RESET FILTERS
            </button>
          )}
        </div>
      )}
    </div>
  );
};
