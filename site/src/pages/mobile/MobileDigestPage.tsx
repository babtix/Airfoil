import React from 'react';
import { Link } from 'react-router-dom';
import type { Story } from '../../types/story';
import { MobileStoryCard } from '../../components/mobile/MobileStoryCard';

interface MobileDigestPageProps {
  stories: Story[];
  onOpenStory: (story: Story) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
}

export const MobileDigestPage: React.FC<MobileDigestPageProps> = ({
  stories,
  onOpenStory,
  selectedTag,
  onSelectTag
}) => {
  const topDigest = [...stories].sort((a, b) => b.score - a.score).slice(0, 5);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      {/* Flight Briefing Card */}
      <div
        className="mobile-card"
        style={{
          cursor: 'default',
          borderLeft: '3px solid var(--kai)',
          padding: '1rem'
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.4rem' }}>
          <span className="mono-label" style={{ color: 'var(--kai)', fontSize: '10px' }}>
            // FLIGHT BRIEFING
          </span>
          <span className="mono-label" style={{ fontSize: '10px' }}>
            TOP 5 SIGNALS
          </span>
        </div>
        <h1 style={{ fontSize: '1.15rem', fontWeight: 700, lineHeight: 1.3, color: 'var(--prim)', margin: '0 0 0.4rem 0' }}>
          Autonomous Coding Benchmarks Shattered, Edge Quantization Matures
        </h1>
        <p style={{ fontSize: '0.825rem', color: 'var(--sec)', lineHeight: 1.5, margin: 0 }}>
          Google DeepMind posts 98% SWE-Bench score while consumer GPUs gain 70B model serving via 4-bit AWQ. Meta open-weights video world model.
        </p>
      </div>

      {/* Top 5 Stories List with rank badges */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
        {topDigest.map((story, index) => (
          <div key={story.id} style={{ display: 'flex', flexDirection: 'column', gap: '0.3rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', paddingLeft: '0.2rem' }}>
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: '11px',
                  fontWeight: 700,
                  color: index === 0 ? 'var(--kai)' : 'var(--sec)'
                }}
              >
                #{index + 1} // SIGNAL {story.score}
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

      {/* Full feed CTA */}
      <div style={{ textAlign: 'center', marginTop: '0.5rem' }}>
        <Link
          to="/feed"
          className="btn-ghost"
          style={{
            display: 'block',
            width: '100%',
            padding: '0.75rem 0',
            textAlign: 'center',
            fontSize: '11px',
            fontWeight: 600
          }}
        >
          VIEW ALL {stories.length} SIGNALS (FEED) →
        </Link>
      </div>
    </div>
  );
};
