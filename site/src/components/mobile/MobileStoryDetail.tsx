import React, { useEffect } from 'react';
import type { Story, SourceType } from '../../types/story';
import { ScoreBadge } from '../ScoreBadge';
import { formatStoryAge } from '../../hooks/useAirfoilSignals';

interface MobileStoryDetailProps {
  story: Story;
  onBack: () => void;
  onSelectTag?: (tag: string) => void;
}

export const MobileStoryDetail: React.FC<MobileStoryDetailProps> = ({
  story,
  onBack,
  onSelectTag
}) => {
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [story.id]);

  const handleShare = async () => {
    if (navigator.share) {
      try {
        await navigator.share({
          title: `Airfoil // ${story.title}`,
          text: story.summary,
          url: window.location.href
        });
      } catch {
        // User cancelled or share failed
      }
    } else {
      navigator.clipboard?.writeText(window.location.href);
      alert('Link copied to clipboard');
    }
  };

  const getSourceBadgeStyle = (type: SourceType) => {
    switch (type) {
      case 'lab':
        return {
          borderColor: 'var(--kai)',
          color: 'var(--kai)',
          backgroundColor: 'rgba(255, 59, 59, 0.08)'
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
    <article style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      {/* Top action bar: Back & Share */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <button
          type="button"
          onClick={onBack}
          className="btn-ghost"
          style={{
            padding: '0.4rem 0.75rem',
            fontSize: '11px',
            display: 'inline-flex',
            alignItems: 'center',
            gap: '0.35rem'
          }}
        >
          ← FEED
        </button>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <button
            type="button"
            onClick={handleShare}
            className="btn-ghost"
            style={{
              padding: '0.4rem 0.6rem',
              fontSize: '11px',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '0.35rem'
            }}
            title="Share Signal"
          >
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="18" cy="5" r="3" />
              <circle cx="6" cy="12" r="3" />
              <circle cx="18" cy="19" r="3" />
              <line x1="8.59" y1="13.51" x2="15.42" y2="17.49" />
              <line x1="15.41" y1="6.51" x2="8.59" y2="10.49" />
            </svg>
            <span>SHARE</span>
          </button>
          <ScoreBadge score={story.score} />
        </div>
      </div>

      {/* Meta header row */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          gap: '0.5rem',
          fontFamily: 'var(--font-mono)',
          fontSize: '11px',
          color: 'var(--sec)'
        }}
      >
        <span>{formatStoryAge(story)}</span>
        <span>·</span>
        <span>CLUSTER ×{story.cluster}</span>
        {story.builder_relevant && (
          <span
            style={{
              padding: '1px 6px',
              borderRadius: '3px',
              border: '1px solid var(--kai-border)',
              color: 'var(--kai)',
              backgroundColor: 'rgba(255, 59, 59, 0.08)',
              fontWeight: 700,
              fontSize: '10px'
            }}
          >
            SHIP
          </span>
        )}
      </div>

      {/* Title */}
      <h1
        style={{
          fontSize: '1.35rem',
          fontWeight: 700,
          lineHeight: 1.3,
          color: 'var(--prim)',
          margin: 0
        }}
      >
        {story.title}
      </h1>

      {/* Tags */}
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem' }}>
        {story.tags.map(tag => (
          <button
            key={tag}
            type="button"
            className="tagpill"
            style={{ padding: '0.25rem 0.55rem', fontSize: '11px' }}
            onClick={() => onSelectTag?.(tag)}
          >
            {tag}
          </button>
        ))}
      </div>

      {/* Highlighted Summary */}
      <div
        style={{
          borderLeft: '3px solid var(--kai)',
          paddingLeft: '0.85rem',
          fontSize: '0.95rem',
          color: 'var(--prim)',
          lineHeight: 1.6,
          backgroundColor: 'rgba(255, 59, 59, 0.03)',
          padding: '0.75rem 0.85rem',
          borderRadius: '0 6px 6px 0'
        }}
      >
        {story.summary}
      </div>

      {/* Takeaways if available */}
      {story.takeaways && story.takeaways.length > 0 && (
        <div
          style={{
            backgroundColor: 'var(--surf)',
            border: '1px solid var(--line)',
            borderRadius: '8px',
            padding: '0.85rem 1rem'
          }}
        >
          <div className="mono-label" style={{ color: 'var(--kai)', marginBottom: '0.5rem' }}>
            // KEY TAKEAWAYS
          </div>
          <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            {story.takeaways.map((item, idx) => (
              <li
                key={idx}
                style={{
                  fontSize: '0.85rem',
                  color: 'var(--prim)',
                  lineHeight: 1.5,
                  display: 'flex',
                  alignItems: 'flex-start',
                  gap: '0.4rem'
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
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
        {story.body.map((paragraph, index) => (
          <p
            key={index}
            style={{
              fontSize: '0.92rem',
              lineHeight: 1.65,
              color: 'var(--prim)',
              margin: 0,
              opacity: 0.92
            }}
          >
            {paragraph}
          </p>
        ))}
      </div>

      {/* Sources List */}
      <div style={{ borderTop: '1px solid var(--line)', paddingTop: '1.25rem', marginTop: '0.5rem' }}>
        <div className="rail-h" style={{ marginBottom: '0.75rem' }}>
          // SOURCES ({story.sources.length})
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
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
                  gap: '0.6rem',
                  padding: '0.65rem 0.75rem',
                  borderRadius: '6px',
                  backgroundColor: 'var(--surf)',
                  border: '1px solid var(--line)',
                  textDecoration: 'none',
                  color: 'inherit',
                  touchAction: 'manipulation'
                }}
              >
                <span
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: '9px',
                    textTransform: 'uppercase',
                    padding: '2px 5px',
                    borderRadius: '3px',
                    border: '1px solid',
                    flexShrink: 0,
                    ...badgeStyle
                  }}
                >
                  {src.type}
                </span>
                <span
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: '12px',
                    color: 'var(--prim)',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    flex: 1
                  }}
                >
                  {src.name} ↗
                </span>
                {src.metrics && (src.metrics.points || src.metrics.upvotes) && (
                  <span
                    style={{
                      fontFamily: 'var(--font-mono)',
                      fontSize: '10px',
                      color: 'var(--kai)',
                      flexShrink: 0
                    }}
                  >
                    ▲ {src.metrics.points || src.metrics.upvotes}
                  </span>
                )}
              </a>
            );
          })}
        </div>
      </div>
    </article>
  );
};
