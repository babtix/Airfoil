import React from 'react';
import { Link } from 'react-router-dom';
import type { Story } from '../types/story';
import { StoryCard } from '../components/StoryCard';
import { ScoreBadge } from '../components/ScoreBadge';
import { AirfoilSVG } from '../components/AirfoilSVG';
import { formatStoryAge } from '../hooks/useAirfoilSignals';

interface HomePageProps {
  stories: Story[];
  topStory: Story | null;
  totalSourcesCount: number;
  onOpenStory: (story: Story) => void;
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
}

export const HomePage: React.FC<HomePageProps> = ({
  stories,
  topStory,
  totalSourcesCount,
  onOpenStory,
  selectedTag,
  onSelectTag
}) => {
  const topSignals = [...stories].sort((a, b) => b.score - a.score).slice(0, 3);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '3.5rem', paddingBottom: '3rem' }}>
      {/* ══════════ HERO SECTION WITH BIG MAIN LOGO ══════════ */}
      <section
        style={{
          border: '1px solid var(--line)',
          borderRadius: '12px',
          backgroundColor: 'var(--surf)',
          padding: 'clamp(2rem, 5vw, 3.5rem)',
          position: 'relative',
          overflow: 'hidden',
          boxShadow: '0 20px 40px -15px rgba(0, 0, 0, 0.7)'
        }}
      >
        {/* Ambient high-G glow behind logo */}
        <div
          style={{
            position: 'absolute',
            top: '20%',
            right: '10%',
            width: '350px',
            height: '350px',
            borderRadius: '50%',
            backgroundColor: 'rgba(255, 59, 59, 0.05)',
            filter: 'blur(80px)',
            pointerEvents: 'none'
          }}
        />

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
            gap: '3rem',
            alignItems: 'center'
          }}
        >
          {/* Hero Left Content */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
            <div
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '0.6rem',
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                color: 'var(--kai)',
                letterSpacing: '0.14em',
                border: '1px solid var(--kai-border)',
                borderRadius: '4px',
                padding: '4px 10px',
                backgroundColor: 'rgba(255, 59, 59, 0.06)',
                width: 'fit-content'
              }}
            >
              <span>● LIVE TELEMETRY</span>
              <span style={{ color: 'var(--sec)' }}>// KAIOKEN FLIGHT DECK</span>
            </div>

            <h1
              style={{
                fontSize: 'clamp(2.2rem, 5vw, 3.4rem)',
                fontWeight: 700,
                lineHeight: 1.12,
                letterSpacing: '-0.03em',
                color: 'var(--prim)'
              }}
            >
              AI SIGNAL TELEMETRY.<br />
              <span style={{ color: 'var(--kai)' }}>READ LIKE A BUILDER.</span>
            </h1>

            <p
              style={{
                fontSize: '1.05rem',
                color: 'var(--sec)',
                lineHeight: 1.65,
                maxWidth: '540px'
              }}
            >
              Airfoil tracks 17 tiered sources, clusters duplicates semantically, and ranks news by mathematical builder utility.
              20 outlets covering one model release becomes <strong>1 unified story</strong>. No hype, no noise, zero-JS static digest.
            </p>

            {/* Launch & Action Surfaces */}
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.75rem', marginTop: '0.5rem' }}>
              <Link
                to="/feed"
                style={{
                  backgroundColor: 'var(--kai)',
                  color: 'var(--deep)',
                  fontFamily: 'var(--font-mono)',
                  fontSize: '12px',
                  fontWeight: 700,
                  letterSpacing: '0.08em',
                  textTransform: 'uppercase',
                  padding: '0.75rem 1.4rem',
                  borderRadius: '4px',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '0.5rem',
                  transition: 'all 0.15s ease',
                  boxShadow: '0 4px 14px rgba(255, 59, 59, 0.3)'
                }}
                className="hero-primary-btn"
              >
                LAUNCH FLIGHT DECK (FEED) →
              </Link>
              <Link to="/digest" className="btn-ghost" style={{ padding: '0.75rem 1.1rem', fontSize: '12px' }}>
                DAILY DIGEST
              </Link>
              <Link to="/ship" className="btn-ghost" style={{ padding: '0.75rem 1.1rem', fontSize: '12px' }}>
                BUILDER CUT (/SHIP)
              </Link>
              <Link to="/top" className="btn-ghost" style={{ padding: '0.75rem 1.1rem', fontSize: '12px' }}>
                LEADERBOARD (/TOP)
              </Link>
            </div>
          </div>

          {/* Hero Right: Prominent Big Tactical Logo */}
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              position: 'relative'
            }}
          >
            <div
              style={{
                width: '100%',
                maxWidth: '360px',
                aspectRatio: '1/1',
                borderRadius: '10px',
                border: '1px solid var(--line)',
                backgroundColor: 'var(--deep)',
                padding: '0.75rem',
                boxShadow: '0 12px 40px rgba(0, 0, 0, 0.8), 0 0 25px rgba(255, 59, 59, 0.1)',
                transition: 'transform 0.2s ease, border-color 0.2s ease'
              }}
              className="main-logo-frame"
            >
              <img
                src="/main%20logo.svg"
                alt="Airfoil Main Tactical Logo"
                style={{
                  width: '100%',
                  height: '100%',
                  display: 'block',
                  borderRadius: '6px'
                }}
              />
            </div>
            <div
              style={{
                marginTop: '1rem',
                display: 'flex',
                alignItems: 'center',
                gap: '0.5rem',
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                color: 'var(--sec)',
                letterSpacing: '0.15em'
              }}
            >
              <span style={{ color: 'var(--kai)' }}>AIRFOIL</span> // TACTICAL LIFT-01 RADAR MARK
            </div>
          </div>
        </div>
      </section>

      {/* ══════════ 4 CORE CAPABILITIES PILLARS ══════════ */}
      <section
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))',
          gap: '1.25rem'
        }}
      >
        <div
          className="card"
          style={{
            cursor: 'default',
            display: 'flex',
            flexDirection: 'column',
            gap: '0.5rem',
            padding: '1.25rem'
          }}
        >
          <div className="mono-label" style={{ color: 'var(--kai)' }}>
            01 // MULTI-SOURCE INGESTION
          </div>
          <h3 style={{ fontSize: '1.1rem', fontWeight: 700, color: 'var(--prim)' }}>
            17 Tiered Feeds
          </h3>
          <p style={{ fontSize: '0.85rem', color: 'var(--sec)', lineHeight: 1.55 }}>
            Tier 1 Official Lab Blogs (Anthropic, DeepMind, Meta), Tier 2 arXiv & HuggingFace, Tier 3 SDKs & Repos, Tier 5 Hacker News & Reddit.
          </p>
        </div>

        <div
          className="card"
          style={{
            cursor: 'default',
            display: 'flex',
            flexDirection: 'column',
            gap: '0.5rem',
            padding: '1.25rem'
          }}
        >
          <div className="mono-label" style={{ color: 'var(--kai)' }}>
            02 // GREEDY CLUSTERING
          </div>
          <h3 style={{ fontSize: '1.1rem', fontWeight: 700, color: 'var(--prim)' }}>
            Cosine Semantic Grouping
          </h3>
          <p style={{ fontSize: '0.85rem', color: 'var(--sec)', lineHeight: 1.55 }}>
            Agglomerative single-link clustering with a 48-hour temporal window. Duplicate coverage collapsed into unified story nodes.
          </p>
        </div>

        <div
          className="card"
          style={{
            cursor: 'default',
            display: 'flex',
            flexDirection: 'column',
            gap: '0.5rem',
            padding: '1.25rem'
          }}
        >
          <div className="mono-label" style={{ color: 'var(--kai)' }}>
            03 // MATHEMATICAL SCORING
          </div>
          <h3 style={{ fontSize: '1.1rem', fontWeight: 700, color: 'var(--prim)' }}>
            Builder Signal Ranking
          </h3>
          <p style={{ fontSize: '0.85rem', color: 'var(--sec)', lineHeight: 1.55 }}>
            Pure deterministic function. Rewards releases, weights, benchmarks, and APIs (+1); penalizes hype buzzwords (-1).
          </p>
        </div>

        <div
          className="card"
          style={{
            cursor: 'default',
            display: 'flex',
            flexDirection: 'column',
            gap: '0.5rem',
            padding: '1.25rem'
          }}
        >
          <div className="mono-label" style={{ color: 'var(--kai)' }}>
            04 // ZERO-SLOP PROSE
          </div>
          <h3 style={{ fontSize: '1.1rem', fontWeight: 700, color: 'var(--prim)' }}>
            Original Summaries
          </h3>
          <p style={{ fontSize: '0.85rem', color: 'var(--sec)', lineHeight: 1.55 }}>
            Hard 300-char excerpt cap. Verbatim overlap rejection rules ensure concise original prose with direct outbound links to all sources.
          </p>
        </div>
      </section>

      {/* ══════════ LIVE MACH LEADER & TELEMETRY STREAM PREVIEW ══════════ */}
      <section style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            borderBottom: '1px solid var(--line)',
            paddingBottom: '0.75rem'
          }}
        >
          <div>
            <span className="mono-label" style={{ color: 'var(--kai)', fontSize: '12px' }}>
              // LIVE TELEMETRY STREAM — TODAY'S HIGHEST SIGNAL POWER
            </span>
          </div>
          <Link
            to="/feed"
            className="mono-label"
            style={{ color: 'var(--sec)', textDecoration: 'underline', textUnderlineOffset: '3px' }}
          >
            ENTER FULL STREAM ({stories.length} SIGNALS) →
          </Link>
        </div>

        {/* Highlight Mach Leader Card */}
        {topStory && (
          <div
            className="card"
            onClick={() => onOpenStory(topStory)}
            style={{
              borderColor: 'var(--kai-border)',
              backgroundColor: 'rgba(255, 59, 59, 0.03)',
              padding: '1.25rem'
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem' }}>
              <span className="mono-label" style={{ color: 'var(--kai)' }}>
                ★ MACH LEADER // TODAY'S TOP SIGNAL
              </span>
              <ScoreBadge score={topStory.score} />
            </div>
            <h2 style={{ fontSize: '1.2rem', fontWeight: 700, lineHeight: 1.35, color: 'var(--prim)', marginBottom: '0.5rem' }}>
              {topStory.title}
            </h2>
            <p style={{ fontSize: '0.9rem', color: 'var(--sec)', lineHeight: 1.55, marginBottom: '0.75rem' }}>
              {topStory.summary}
            </p>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div style={{ display: 'flex', gap: '0.4rem' }}>
                {topStory.tags.map(t => (
                  <span key={t} className="tagpill tagpill-on">{t}</span>
                ))}
              </div>
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--sec)' }}>
                {formatStoryAge(topStory)} · CLUSTER ×{topStory.cluster} · {topStory.sources.length} SOURCES ↗
              </span>
            </div>
          </div>
        )}

        {/* Other leading signals */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {topSignals.slice(1).map(story => (
            <StoryCard
              key={story.id}
              story={story}
              onOpenStory={onOpenStory}
              selectedTag={selectedTag}
              onSelectTag={onSelectTag}
            />
          ))}
        </div>
      </section>

      {/* ══════════ DATA PIPELINE & AERODYNAMICS ══════════ */}
      <section
        style={{
          border: '1px solid var(--line)',
          borderRadius: '12px',
          backgroundColor: 'var(--surf)',
          padding: 'clamp(1.5rem, 4vw, 2rem)',
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
          gap: '2rem',
          alignItems: 'center'
        }}
      >
        <div>
          <div className="mono-label" style={{ color: 'var(--kai)', marginBottom: '0.5rem' }}>
            // FIG.01 — SIGNAL AERODYNAMICS
          </div>
          <h3 style={{ fontSize: '1.3rem', fontWeight: 700, color: 'var(--prim)', marginBottom: '0.75rem' }}>
            Converting Raw AI Noise into Aerodynamic Lift
          </h3>
          <p style={{ fontSize: '0.9rem', color: 'var(--sec)', lineHeight: 1.65, marginBottom: '1.25rem' }}>
            Just as an airfoil cross-section channels airflow to produce lift, Airfoil normalizes, embeds, and ranks multi-source telemetry to amplify signal while discarding hype.
          </p>

          <div
            style={{
              fontFamily: 'var(--font-mono)',
              fontSize: '11px',
              backgroundColor: 'var(--deep)',
              border: '1px solid var(--line)',
              borderRadius: '6px',
              padding: '0.85rem 1rem',
              lineHeight: 1.7,
              color: 'var(--prim)'
            }}
          >
            <div><span style={{ color: 'var(--kai)' }}>1. INGEST:</span> {totalSourcesCount} public feeds & APIs</div>
            <div><span style={{ color: 'var(--kai)' }}>2. NORMALIZE:</span> Max 300-char excerpt hard cap</div>
            <div><span style={{ color: 'var(--kai)' }}>3. CLUSTER:</span> Greedy agglomerative cosine ≥ 0.82</div>
            <div><span style={{ color: 'var(--kai)' }}>4. SCORE:</span> Source tiers + builder signals - hype decay</div>
            <div><span style={{ color: 'var(--kai)' }}>5. SHIP:</span> Static JSON/Markdown deployment</div>
          </div>
        </div>

        <div
          style={{
            border: '1px solid var(--line)',
            borderRadius: '8px',
            backgroundColor: 'var(--deep)',
            padding: '1rem',
            boxShadow: '0 8px 24px rgba(0, 0, 0, 0.4)'
          }}
        >
          <AirfoilSVG />
        </div>
      </section>

      {/* ══════════ BOTTOM CTA & FLIGHT LAUNCH ══════════ */}
      <section
        style={{
          border: '1px solid var(--line)',
          borderRadius: '12px',
          backgroundColor: 'var(--surf)',
          padding: '2.5rem 1.5rem',
          textAlign: 'center',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: '1.25rem'
        }}
      >
        <div className="mono-label" style={{ color: 'var(--kai)' }}>
          // READY TO COMMENCE FLIGHT
        </div>
        <h2 style={{ fontSize: '1.75rem', fontWeight: 700, color: 'var(--prim)', maxWidth: '600px', lineHeight: 1.25 }}>
          Explore the Full Stream of Real-Time AI News & Benchmarks
        </h2>
        <p style={{ fontSize: '0.95rem', color: 'var(--sec)', maxWidth: '500px', lineHeight: 1.6 }}>
          Filter by date window, technology tags, and source tiers. Read without ads or tracking.
        </p>
        <Link
          to="/feed"
          style={{
            backgroundColor: 'var(--kai)',
            color: 'var(--deep)',
            fontFamily: 'var(--font-mono)',
            fontSize: '13px',
            fontWeight: 700,
            letterSpacing: '0.08em',
            textTransform: 'uppercase',
            padding: '0.85rem 1.75rem',
            borderRadius: '4px',
            display: 'inline-flex',
            alignItems: 'center',
            gap: '0.5rem',
            boxShadow: '0 4px 20px rgba(255, 59, 59, 0.35)'
          }}
          className="hero-primary-btn"
        >
          ENTER AIRFOIL FLIGHT DECK (/FEED) →
        </Link>
      </section>

      <style>{`
        .hero-primary-btn:hover {
          background-color: var(--prim) !important;
          box-shadow: 0 6px 25px rgba(245, 245, 245, 0.3) !important;
        }
        .main-logo-frame:hover {
          border-color: var(--kai-border) !important;
          transform: translateY(-4px);
        }
      `}</style>
    </div>
  );
};
