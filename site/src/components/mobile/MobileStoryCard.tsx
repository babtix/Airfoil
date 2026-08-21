import React from 'react';
import type { Story } from '../../types/story';
import { ScoreBadge } from '../ScoreBadge';
import { formatStoryAge } from '../../hooks/useAirfoilSignals';

interface MobileStoryCardProps {
  story: Story;
  onOpenStory: (story: Story) => void;
  selectedTag?: string | null;
  onSelectTag?: (tag: string) => void;
}

export const MobileStoryCard: React.FC<MobileStoryCardProps> = ({
  story,
  onOpenStory,
  selectedTag,
  onSelectTag
}) => {
  return (
    <article
      className="mobile-card"
      onClick={() => onOpenStory(story)}
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: '0.65rem'
      }}
    >
      {/* Top row: Title + Score badge */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '0.75rem' }}>
        <h2
          style={{
            fontSize: '1rem',
            fontWeight: 700,
            lineHeight: 1.35,
            color: 'var(--prim)',
            margin: 0
          }}
        >
          {story.title}
        </h2>
        <ScoreBadge score={story.score} />
      </div>

      {/* Clamped summary */}
      <p
        className="line-clamp-2"
        style={{
          fontSize: '0.85rem',
          color: 'var(--sec)',
          lineHeight: 1.5,
          margin: 0
        }}
      >
        {story.summary}
      </p>

      {/* Tags row and footer telemetry */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          gap: '0.35rem',
          paddingTop: '0.2rem'
        }}
      >
        {story.tags.slice(0, 3).map(tag => (
          <button
            key={tag}
            type="button"
            className={`tagpill ${selectedTag === tag ? 'tagpill-on' : ''}`}
            style={{ fontSize: '10px', padding: '2px 6px' }}
            onClick={e => {
              e.stopPropagation();
              onSelectTag?.(tag);
            }}
          >
            {tag}
          </button>
        ))}

        {story.builder_relevant && (
          <span
            style={{
              fontFamily: 'var(--font-mono)',
              fontSize: '9px',
              padding: '1px 5px',
              borderRadius: '3px',
              border: '1px solid var(--kai-border)',
              color: 'var(--kai)',
              backgroundColor: 'rgba(255, 59, 59, 0.08)',
              fontWeight: 700
            }}
          >
            SHIP
          </span>
        )}

        <div
          style={{
            marginLeft: 'auto',
            fontFamily: 'var(--font-mono)',
            fontSize: '10px',
            color: 'var(--sec)',
            letterSpacing: '0.02em',
            whiteSpace: 'nowrap'
          }}
        >
          {formatStoryAge(story)} · {story.sources.length} SRC
        </div>
      </div>
    </article>
  );
};
