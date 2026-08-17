import React, { useEffect } from 'react';
import type { Story, SourceType } from '../types/story';
import { ScoreBadge } from './ScoreBadge';
import { TagPill } from './TagPill';
import { formatStoryAge } from '../hooks/useAirfoilSignals';

interface StoryDetailProps {
  story: Story;
  onBack: () => void;
  onSelectTag?: (tag: string) => void;
}

export const StoryDetail: React.FC<StoryDetailProps> = ({ story, onBack, onSelectTag }) => {
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [story.id]);

  const getSourceBadgeStyle = (type: SourceType) => {
    switch (type) {
      case 'lab':
        return {
          borderColor: 'var(--kai)',
          color: 'var(--kai)',
          backgroundColor: 'rgba(255, 59, 59, 0.05)'
        };
      case 'community':
        return {
          borderColor: 'var(--sec)',
          color: 'var(--sec)',
          backgroundColor: 'transparent'
        };
      case 'press':
      default:
        return {
          borderColor: 'var(--line)',
          color: 'var(--sec)',
          backgroundColor: 'transparent'
        };
    }
  };

  return (
    <article style={{ maxWidth: '800px', margin: '0 auto', paddingBottom: '2rem' }}>
      {/* Back button */}
      <button
        type="button"
        onClick={onBack}
        className="btn-ghost"
        style={{ marginBottom: '1.5rem', display: 'inline-flex', alignItems: 'center', gap: '0.4rem' }}
      >
        ← BACK TO FEED
      </button>

      {/* Meta Bar */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '1rem' }}>
        <ScoreBadge score={story.score} />
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--sec)', letterSpacing: '0.05em' }}>
          {formatStoryAge(story)} · CLUSTER ×{story.cluster}
        </span>
        {story.builder_relevant && (
          <span
            style={{
              fontFamily: 'var(--font-mono)',
              fontSize: '10px',
              padding: '1px 6px',
              borderRadius: '3px',
              border: '1px solid var(--kai-border)',
              color: 'var(--kai)',
              backgroundColor: 'rgba(255, 59, 59, 0.08)',
              fontWeight: 600
            }}
          >
            SHIP // BUILDER SIGNAL
          </span>
        )}
      </div>

      {/* Title */}
      <h1
        style={{
          fontWeight: 700,
          fontSize: 'clamp(1.5rem, 4vw, 2.3rem)',
          lineHeight: 1.25,
          marginBottom: '1.25rem',
          color: 'var(--prim)'
        }}
      >
        {story.title}
      </h1>

      {/* Tags */}
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.375rem', marginBottom: '1.75rem' }}>
        {story.tags.map(tag => (
          <TagPill
            key={tag}
            tag={tag}
            onClick={() => onSelectTag?.(tag)}
          />
        ))}
      </div>

      {/* Summary Highlight */}
      <div
        style={{
          borderLeft: '2px solid var(--kai)',
          paddingLeft: '1rem',
          marginBottom: '1.5rem',
          fontSize: '1.05rem',
          fontStyle: 'italic',
          color: 'var(--prim)',
          lineHeight: 1.6
        }}
      >
        {story.summary}
      </div>

      {/* Takeaways / Key Points if available */}
      {story.takeaways && story.takeaways.length > 0 && (
        <div
          style={{
            backgroundColor: 'var(--surf)',
            border: '1px solid var(--line)',
            borderRadius: '6px',
            padding: '1rem 1.25rem',
            marginBottom: '1.75rem'
          }}
        >
          <div className="mono-label" style={{ color: 'var(--kai)', marginBottom: '0.5rem' }}>
            // KEY TAKEAWAYS
          </div>
          <ul style={{ listStyle: 'none', display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            {story.takeaways.map((item, idx) => (
              <li
                key={idx}
                style={{
                  fontSize: '0.9rem',
                  color: 'var(--prim)',
                  lineHeight: 1.5,
                  display: 'flex',
                  alignItems: 'flex-start',
                  gap: '0.5rem'
                }}
              >
                <span style={{ color: 'var(--kai)', fontFamily: 'var(--font-mono)' }}>›</span>
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Body paragraphs */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', marginBottom: '2.5rem' }}>
        {story.body.map((paragraph, index) => (
          <p
            key={index}
            style={{
              fontSize: '1rem',
              lineHeight: 1.68,
              color: 'rgba(245, 245, 245, 0.92)'
            }}
          >
            {paragraph}
          </p>
        ))}
      </div>

      {/* Sources Section */}
      <div style={{ borderTop: '1px solid var(--line)', paddingTop: '1.5rem' }}>
        <div className="rail-h">// SOURCES ({story.sources.length})</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
          {story.sources.map((src, index) => {
            const badgeStyle = getSourceBadgeStyle(src.type);
            return (
              <a
                key={index}
                href={src.url}
                target="_blank"
                rel="noopener noreferrer"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.75rem',
                  padding: '0.4rem 0'
                }}
                className="source-link group"
              >
                <span
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: '10px',
                    textTransform: 'uppercase',
                    padding: '2px 6px',
                    borderRadius: '2px',
                    border: '1px solid',
                    ...badgeStyle
                  }}
                >
                  {src.type}
                </span>
                <span
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: '13px',
                    color: 'var(--sec)',
                    textDecoration: 'underline',
                    textDecorationStyle: 'dotted',
                    textUnderlineOffset: '4px',
                    transition: 'color 0.15s ease'
                  }}
                  className="source-title"
                >
                  {src.name} ↗
                </span>
                {src.metrics && (src.metrics.points || src.metrics.upvotes) && (
                  <span style={{ marginLeft: 'auto', fontFamily: 'var(--font-mono)', fontSize: '10px', color: 'var(--sec)' }}>
                    ▲ {src.metrics.points || src.metrics.upvotes}
                  </span>
                )}
              </a>
            );
          })}
        </div>
      </div>

      <style>{`
        .source-link:hover .source-title {
          color: var(--kai) !important;
        }
      `}</style>
    </article>
  );
};
