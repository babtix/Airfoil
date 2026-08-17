import React from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import type { Story } from '../types/story';
import { StoryDetail } from '../components/StoryDetail';

interface StoryPageProps {
  stories: Story[];
  onSelectTag?: (tag: string) => void;
}

export const StoryPage: React.FC<StoryPageProps> = ({ stories, onSelectTag }) => {
  const { slug } = useParams<{ slug: string }>();
  const navigate = useNavigate();

  const story = stories.find(s => s.slug === slug || s.id === slug);

  if (!story) {
    return (
      <div
        style={{
          border: '1px solid var(--line)',
          borderRadius: '6px',
          padding: '3rem 1.5rem',
          textAlign: 'center',
          backgroundColor: 'var(--surf)'
        }}
      >
        <p className="mono-label" style={{ marginBottom: '1rem', color: 'var(--kai)' }}>
          SIGNAL NOT FOUND // INVALID TELEMETRY ID
        </p>
        <button
          type="button"
          onClick={() => navigate('/feed')}
          className="btn-ghost"
        >
          ← RETURN TO FLIGHT DECK
        </button>
      </div>
    );
  }

  return (
    <StoryDetail
      story={story}
      onBack={() => navigate(-1)}
      onSelectTag={tag => {
        onSelectTag?.(tag);
        navigate('/feed');
      }}
    />
  );
};
