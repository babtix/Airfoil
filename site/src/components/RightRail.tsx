import React, { useState } from 'react';
import type { Story } from '../types/story';
import { ScoreBadge } from './ScoreBadge';
import { AirfoilSVG } from './AirfoilSVG';
import { formatStoryAge } from '../hooks/useAirfoilSignals';

interface RightRailProps {
  topStory: Story | null;
  onOpenStory: (story: Story) => void;
}

export const RightRail: React.FC<RightRailProps> = ({ topStory, onOpenStory }) => {
  const [email, setEmail] = useState('');
  const [isSubscribed, setIsSubscribed] = useState(false);

  const handleSubscribe = (e: React.FormEvent) => {
    e.preventDefault();
    if (email.trim()) {
      setIsSubscribed(true);
    }
  };

  return (
    <aside style={{ width: '100%' }}>
      {/* Mach Leader Top Story Card */}
      {topStory && (
        <div style={{ marginBottom: '1.5rem' }}>
          <div className="rail-h">// TODAY'S TOP SCORE</div>
          <div
            className="card"
            onClick={() => onOpenStory(topStory)}
            style={{ cursor: 'pointer' }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: '0.5rem', marginBottom: '0.5rem' }}>
              <span className="mono-label" style={{ color: 'var(--kai)' }}>MACH LEADER</span>
              <ScoreBadge score={topStory.score} />
            </div>
            <p style={{ fontWeight: 700, lineHeight: 1.35, marginBottom: '0.35rem', color: 'var(--prim)', fontSize: '0.95rem' }}>
              {topStory.title}
            </p>
            <p style={{ fontFamily: 'var(--font-mono)', fontSize: '10px', color: 'var(--sec)' }}>
              {formatStoryAge(topStory)} · CLUSTER ×{topStory.cluster}
            </p>
          </div>
        </div>
      )}

      {/* Aerodynamics SVG Diagram */}
      <div style={{ marginBottom: '1.5rem' }}>
        <div className="rail-h">// FIG.01 — SIGNAL AERODYNAMICS</div>
        <div
          style={{
            border: '1px solid var(--line)',
            borderRadius: '6px',
            backgroundColor: 'var(--surf)',
            padding: '0.5rem'
          }}
        >
          <AirfoilSVG />
        </div>
      </div>

      {/* Newsletter Engagement Form */}
      <div style={{ marginBottom: '1.5rem' }}>
        <div className="rail-h">// NEWSLETTER</div>
        {isSubscribed ? (
          <div
            style={{
              border: '1px solid var(--kai-border)',
              borderRadius: '6px',
              padding: '0.75rem',
              fontFamily: 'var(--font-mono)',
              fontSize: '11px',
              color: 'var(--kai)',
              backgroundColor: 'rgba(255, 59, 59, 0.05)',
              lineHeight: 1.4
            }}
          >
            CONFIRMED // DIGEST ARMED FOR {email.toUpperCase()}
          </div>
        ) : (
          <form onSubmit={handleSubscribe}>
            <input
              type="email"
              required
              value={email}
              onChange={e => setEmail(e.target.value)}
              placeholder="pilot@base.io"
              style={{
                width: '100%',
                backgroundColor: 'var(--surf)',
                border: '1px solid var(--line)',
                borderRadius: '4px',
                padding: '0.5rem 0.75rem',
                fontFamily: 'var(--font-mono)',
                fontSize: '12px',
                color: 'var(--prim)',
                marginBottom: '0.5rem'
              }}
            />
            <button
              type="submit"
              style={{
                width: '100%',
                backgroundColor: 'var(--kai)',
                color: 'var(--deep)',
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                fontWeight: 600,
                letterSpacing: '0.1em',
                textTransform: 'uppercase',
                borderRadius: '4px',
                padding: '0.5rem 0.75rem',
                transition: 'background-color 0.15s ease'
              }}
              className="engage-btn"
            >
              Engage — Daily Digest
            </button>
          </form>
        )}
      </div>

      {/* About Box */}
      <div>
        <div className="rail-h">// ABOUT</div>
        <p style={{ fontSize: '13px', color: 'var(--sec)', lineHeight: 1.6 }}>
          Airfoil ingests 17 tiered sources, clusters duplicates, scores signal strength, and ships a static digest. No ads, no tracking, zero JS shipped to readers.
        </p>
      </div>

      <style>{`
        .engage-btn:hover {
          background-color: var(--prim) !important;
        }
      `}</style>
    </aside>
  );
};
