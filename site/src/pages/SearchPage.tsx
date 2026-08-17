import React, { useRef, useEffect } from 'react';
import type { Story } from '../types/story';
import { StoryCard } from '../components/StoryCard';

interface SearchPageProps {
  stories: Story[];
  query: string;
  onQueryChange: (q: string) => void;
  onOpenStory: (story: Story) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
  tags: string[];
}

export const SearchPage: React.FC<SearchPageProps> = ({
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
    inputRef.current?.focus();
  }, []);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      {/* Search Bar Container */}
      <div
        style={{
          border: '1px solid var(--line)',
          borderRadius: '6px',
          backgroundColor: 'var(--surf)',
          padding: '1.25rem'
        }}
      >
        <div className="mono-label" style={{ color: 'var(--kai)', marginBottom: '0.75rem' }}>
          // RADAR QUERY // SIGNAL RECONNAISSANCE
        </div>
        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={e => onQueryChange(e.target.value)}
            placeholder="> enter keywords, benchmarks, model names, or tags…"
            style={{
              width: '100%',
              backgroundColor: 'var(--deep)',
              border: '1px solid var(--line)',
              borderRadius: '4px',
              padding: '0.65rem 0.85rem',
              fontFamily: 'var(--font-mono)',
              fontSize: '0.95rem',
              color: 'var(--prim)'
            }}
          />
          {query && (
            <button
              type="button"
              onClick={() => onQueryChange('')}
              className="btn-ghost"
              style={{ flexShrink: 0, padding: '0.55rem 0.8rem' }}
            >
              CLEAR
            </button>
          )}
        </div>

        {/* Quick Tag suggestions */}
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem', marginTop: '0.85rem' }}>
          <span className="mono-label" style={{ alignSelf: 'center', marginRight: '0.25rem', fontSize: '9px' }}>
            HOT TAGS:
          </span>
          {tags.slice(0, 8).map(tag => (
            <button
              key={tag}
              type="button"
              onClick={() => onSelectTag(tag)}
              className={`tagpill ${selectedTag === tag ? 'tagpill-on' : ''}`}
            >
              {tag}
            </button>
          ))}
        </div>
      </div>

      {/* Result Status */}
      <div className="mono-label" style={{ fontSize: '10px' }}>
        {query
          ? `MATCHED ${stories.length} SIGNALS FOR "${query.toUpperCase()}"`
          : `DISPLAYING ALL ${stories.length} ACTIVE SIGNALS`}
      </div>

      {/* Search results list */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
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
              NO SIGNALS MATCHED YOUR QUERY
            </p>
            <button
              type="button"
              onClick={() => onQueryChange('')}
              className="btn-ghost btn-ghost-on"
            >
              CLEAR SEARCH QUERY
            </button>
          </div>
        )}
      </div>
    </div>
  );
};
