import React from 'react';
import type { Story } from '../types/story';
import { ScoreBadge } from './ScoreBadge';
import { TagPill } from './TagPill';
import { formatStoryAge } from '../hooks/useAirfoilSignals';

interface StoryCardProps {
  story: Story;
  onOpenStory: (story: Story) => void;
  selectedTag?: string | null;
  onSelectTag?: (tag: string) => void;
}

export const StoryCard: React.FC<StoryCardProps> = ({
  story,
  onOpenStory,
  selectedTag,
  onSelectTag
}) => {
  return (
    <article
      className="card"
      onClick={() => onOpenStory(story)}
      style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}
    >
      {/* Title & Score Badge */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '0.75rem' }}>
        <h2 style={{ fontSize: '1.05rem', fontWeight: 700, lineHeight: 1.35, color: 'var(--prim)' }}>
          {story.title}
        </h2>
        <ScoreBadge score={story.score} />
      </div>

      {/* Summary */}
      <p className="line-clamp-2" style={{ fontSize: '0.875rem', color: 'var(--sec)', lineHeight: 1.55 }}>
        {story.summary}
      </p>

      {/* Tags and Meta row */}
      <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '0.375rem', marginTop: '0.25rem' }}>
        {story.tags.map(tag => (
          <TagPill
            key={tag}
            tag={tag}
            isActive={selectedTag === tag}
            onClick={e => {
              e.stopPropagation();
              onSelectTag?.(tag);
            }}
          />
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
              fontWeight: 600
            }}
            title="Builder Relevant Signal"
          >
            SHIP
          </span>
        )}

        <span
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
        </span>
      </div>
    </article>
  );
};
