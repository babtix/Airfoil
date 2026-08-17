import React from 'react';
import type { Story } from '../types/story';
import { StoryCard } from '../components/StoryCard';
import { Link } from 'react-router-dom';

interface DigestPageProps {
  stories: Story[];
  onOpenStory: (story: Story) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
}

export const DigestPage: React.FC<DigestPageProps> = ({
  stories,
  onOpenStory,
  selectedTag,
  onSelectTag
}) => {
  // Top 5 stories sorted by score
  const topDigest = [...stories].sort((a, b) => b.score - a.score).slice(0, 5);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      {/* Digest Header Telemetry Banner */}
      <div
        style={{
          border: '1px solid var(--line)',
          borderRadius: '6px',
          backgroundColor: 'var(--surf)',
          padding: '1.25rem'
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem' }}>
          <span className="mono-label" style={{ color: 'var(--kai)' }}>
            // TODAY'S FLIGHT BRIEFING
          </span>
          <span className="mono-label" style={{ fontSize: '10px' }}>
            TOP 5 SIGNALS
          </span>
        </div>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, lineHeight: 1.3, marginBottom: '0.5rem', color: 'var(--prim)' }}>
          Autonomous Coding Benchmarks Shattered, Edge Quantization Matures
        </h1>
        <p style={{ fontSize: '0.875rem', color: 'var(--sec)', lineHeight: 1.55 }}>
          Google DeepMind posts 98% SWE-Bench score while consumer GPUs gain 70B model serving via 4-bit AWQ. Meta open-weights video world model.
        </p>
      </div>

      {/* Stories list */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
        {topDigest.map(story => (
          <StoryCard
            key={story.id}
            story={story}
            onOpenStory={onOpenStory}
            selectedTag={selectedTag}
            onSelectTag={onSelectTag}
          />
        ))}
      </div>

      {/* Link to full feed */}
      <div style={{ textAlign: 'center', marginTop: '1rem' }}>
        <Link to="/feed" className="btn-ghost" style={{ padding: '0.5rem 1rem' }}>
          VIEW FULL STREAM ({stories.length} SIGNALS) →
        </Link>
      </div>
    </div>
  );
};
