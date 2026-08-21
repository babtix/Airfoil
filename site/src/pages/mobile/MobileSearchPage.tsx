import React, { useRef, useEffect } from 'react';
import type { Story } from '../../types/story';
import { MobileStoryCard } from '../../components/mobile/MobileStoryCard';

interface MobileSearchPageProps {
  stories: Story[];
  query: string;
  onQueryChange: (q: string) => void;
  onOpenStory: (story: Story) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
  tags: string[];
}

export const MobileSearchPage: React.FC<MobileSearchPageProps> = ({
  stories,
  query,
  onQueryChange,
  onOpenStory,
  selectedTag,
  onSelectTag,
  tags
}) => {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    // Focus search input on mount
    inputRef.current?.focus();
  }, []);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
      {/* Search Input Card */}
      <div
        className="mobile-card"
        style={{
          cursor: 'default',
          padding: '0.85rem 1rem',
          display: 'flex',
          flexDirection: 'column',
          gap: '0.75rem'
        }}
      >
        <div className="mono-label" style={{ color: 'var(--kai)', fontSize: '10px' }}>
          // RADAR QUERY // SEARCH SIGNALS
        </div>

        <div style={{ display: 'flex', gap: '0.4rem', alignItems: 'center' }}>
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={e => onQueryChange(e.target.value)}
            placeholder="> enter model, paper, tag…"
            style={{
              width: '100%',
              backgroundColor: 'var(--deep)',
              border: '1px solid var(--line)',
              borderRadius: '6px',
              padding: '0.6rem 0.75rem',
              fontFamily: 'var(--font-mono)',
              fontSize: '13px',
              color: 'var(--prim)'
            }}
          />
          {query && (
            <button
              type="button"
              onClick={() => onQueryChange('')}
              className="btn-ghost"
              style={{ flexShrink: 0, padding: '0.5rem 0.65rem', fontSize: '11px' }}
            >
              CLEAR
            </button>
          )}
        </div>

        {/* Hot Tags horizontal scroller */}
        <div>
          <div className="mono-label" style={{ fontSize: '9px', marginBottom: '0.35rem' }}>
            SUGGESTED TAGS:
          </div>
          <div className="mobile-pill-scroller">
            {tags.slice(0, 10).map(tag => (
              <button
                key={tag}
                type="button"
                onClick={() => onSelectTag(tag)}
                className={`tagpill ${selectedTag === tag ? 'tagpill-on' : ''}`}
                style={{ padding: '3px 8px', fontSize: '10px' }}
              >
                {tag}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Query Status */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          padding: '0 0.2rem'
        }}
      >
        <span className="mono-label" style={{ fontSize: '10px' }}>
          {query
            ? `MATCHED ${stories.length} SIGNALS FOR "${query.toUpperCase()}"`
            : `SHOWING ALL ${stories.length} SIGNALS`}
        </span>
      </div>

      {/* Results List */}
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
            NO SIGNALS MATCHED YOUR QUERY
          </p>
          <button
            type="button"
            onClick={() => onQueryChange('')}
            className="btn-ghost btn-ghost-on"
            style={{ padding: '0.5rem 1rem', fontSize: '11px' }}
          >
            CLEAR SEARCH
          </button>
        </div>
      )}
    </div>
  );
};
