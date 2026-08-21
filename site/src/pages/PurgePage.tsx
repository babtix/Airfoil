import React, { useState, useMemo } from 'react';
import type { Story } from '../types/story';

interface PurgePageProps {
  stories: Story[];
  allTags: string[];
  topSources: [string, number][];
  onOpenStory: (story: Story) => void;
}

type AgePreset = 'ALL' | '7D' | '30D' | '60D' | '90D' | '180D' | '1Y';

export const PurgePage: React.FC<PurgePageProps> = ({
  stories,
  allTags,
  topSources,
  onOpenStory
}) => {
  // Filter States
  const [agePreset, setAgePreset] = useState<AgePreset>('90D');
  const [customDays, setCustomDays] = useState<number>(90);
  const [useCustomAge, setUseCustomAge] = useState<boolean>(false);
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [tagSearch, setTagSearch] = useState<string>('');
  const [selectedSource, setSelectedSource] = useState<string>('ALL');
  const [selectedTier, setSelectedTier] = useState<string>('ALL');
  const [maxScore, setMaxScore] = useState<number>(100);
  const [filterByScore, setFilterByScore] = useState<boolean>(false);
  const [query, setQuery] = useState<string>('');
  const [sortBy, setSortBy] = useState<'oldest' | 'newest' | 'score_asc' | 'score_desc'>('oldest');

  // Manual Selection state (map of story ID / slug -> boolean)
  const [selectedSlugs, setSelectedSlugs] = useState<Record<string, boolean>>({});
  const [isCopiedJson, setIsCopiedJson] = useState<boolean>(false);
  const [isCopiedCli, setIsCopiedCli] = useState<boolean>(false);
  const [includeDryRunInCli, setIncludeDryRunInCli] = useState<boolean>(true);

  // Compute cutoff timestamp based on age filter
  const cutoffTime = useMemo(() => {
    if (useCustomAge) {
      return Date.now() - customDays * 24 * 60 * 60 * 1000;
    }
    const daysMap: Record<AgePreset, number | null> = {
      ALL: null,
      '7D': 7,
      '30D': 30,
      '60D': 60,
      '90D': 90,
      '180D': 180,
      '1Y': 365
    };
    const days = daysMap[agePreset];
    if (days === null) return null;
    return Date.now() - days * 24 * 60 * 60 * 1000;
  }, [agePreset, useCustomAge, customDays]);

  // Compute Matching stories
  const matchingStories = useMemo(() => {
    return stories.filter(s => {
      const storyTime = new Date(s.ts).getTime();

      // Age cutoff filter
      if (cutoffTime !== null && storyTime >= cutoffTime) {
        return false;
      }

      // Tags filter (match if story has ANY selected tag)
      if (selectedTags.length > 0) {
        const hasTag = selectedTags.some(t => s.tags.includes(t));
        if (!hasTag) return false;
      }

      // Source filter
      if (selectedSource !== 'ALL') {
        const hasSource = s.sources.some(src => src.name.toLowerCase() === selectedSource.toLowerCase());
        if (!hasSource) return false;
      }

      // Tier filter
      if (selectedTier !== 'ALL') {
        // Fallback tier detection from score/cluster or properties
        const storyTier = (s as any).tier || (s.score >= 50 ? 'major' : s.score >= 25 ? 'notable' : 'minor');
        if (storyTier !== selectedTier) return false;
      }

      // Score filter
      if (filterByScore && s.score > maxScore) {
        return false;
      }

      // Query filter
      if (query.trim()) {
        const q = query.toLowerCase();
        const content = `${s.title} ${s.summary} ${s.tags.join(' ')}`.toLowerCase();
        if (!content.includes(q)) return false;
      }

      return true;
    }).sort((a, b) => {
      const tA = new Date(a.ts).getTime();
      const tB = new Date(b.ts).getTime();
      if (sortBy === 'oldest') return tA - tB;
      if (sortBy === 'newest') return tB - tA;
      if (sortBy === 'score_asc') return a.score - b.score;
      if (sortBy === 'score_desc') return b.score - a.score;
      return 0;
    });
  }, [stories, cutoffTime, selectedTags, selectedSource, selectedTier, filterByScore, maxScore, query, sortBy]);

  // Determine active selection list
  // If user hasn't explicitly unselected anything in matching, all matching stories are considered selected
  const activeSelectedStories = useMemo(() => {
    return matchingStories.filter(s => {
      if (selectedSlugs[s.slug] !== undefined) {
        return selectedSlugs[s.slug];
      }
      return true; // Default selected when matching
    });
  }, [matchingStories, selectedSlugs]);

  const selectedCount = activeSelectedStories.length;

  // Toggle selection
  const handleToggleStory = (slug: string) => {
    setSelectedSlugs(prev => {
      const current = prev[slug] !== undefined ? prev[slug] : true;
      return { ...prev, [slug]: !current };
    });
  };

  const handleSelectAllMatching = () => {
    const updated: Record<string, boolean> = {};
    matchingStories.forEach(s => {
      updated[s.slug] = true;
    });
    setSelectedSlugs(updated);
  };

  const handleDeselectAll = () => {
    const updated: Record<string, boolean> = {};
    matchingStories.forEach(s => {
      updated[s.slug] = false;
    });
    setSelectedSlugs(updated);
  };

  const handleInvertSelection = () => {
    const updated: Record<string, boolean> = {};
    matchingStories.forEach(s => {
      const current = selectedSlugs[s.slug] !== undefined ? selectedSlugs[s.slug] : true;
      updated[s.slug] = !current;
    });
    setSelectedSlugs(updated);
  };

  const handleToggleTag = (tag: string) => {
    setSelectedTags(prev =>
      prev.includes(tag) ? prev.filter(t => t !== tag) : [...prev, tag]
    );
  };

  // Copy manifest to clipboard
  const handleCopyManifest = () => {
    const slugs = activeSelectedStories.map(s => s.slug);
    const json = JSON.stringify(slugs, null, 2);
    navigator.clipboard.writeText(json).then(() => {
      setIsCopiedJson(true);
      setTimeout(() => setIsCopiedJson(false), 2500);
    });
  };

  // Download manifest JSON file
  const handleDownloadManifest = () => {
    const slugs = activeSelectedStories.map(s => s.slug);
    const blob = new Blob([JSON.stringify(slugs, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `purge-manifest-${new Date().toISOString().slice(0, 10)}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  // Build CLI command string
  const cliCommand = useMemo(() => {
    let cmd = './agent/bin/airfoil purge';
    
    if (useCustomAge) {
      cmd += ` --before ${customDays}d`;
    } else if (agePreset !== 'ALL') {
      const ageFlag = agePreset.toLowerCase();
      cmd += ` --before ${ageFlag}`;
    }

    if (selectedTier !== 'ALL') {
      cmd += ` --tier ${selectedTier}`;
    }

    if (filterByScore) {
      cmd += ` --max-score ${maxScore}`;
    }

    if (selectedTags.length > 0) {
      selectedTags.forEach(t => {
        cmd += ` --tag "${t}"`;
      });
    }

    if (selectedSource !== 'ALL') {
      cmd += ` --source "${selectedSource}"`;
    }

    if (query.trim()) {
      cmd += ` --query "${query.trim()}"`;
    }

    if (includeDryRunInCli) {
      cmd += ' --dry';
    }

    return cmd;
  }, [useCustomAge, customDays, agePreset, selectedTier, filterByScore, maxScore, selectedTags, selectedSource, query, includeDryRunInCli]);

  const handleCopyCli = (textToCopy?: string) => {
    navigator.clipboard.writeText(textToCopy || cliCommand).then(() => {
      setIsCopiedCli(true);
      setTimeout(() => setIsCopiedCli(false), 2500);
    });
  };

  const filteredTagsList = useMemo(() => {
    if (!tagSearch.trim()) return allTags.slice(0, 24);
    return allTags
      .filter(t => t.toLowerCase().includes(tagSearch.toLowerCase()))
      .slice(0, 30);
  }, [allTags, tagSearch]);

  const earliestDate = matchingStories.length > 0
    ? new Date(Math.min(...matchingStories.map(s => new Date(s.ts).getTime()))).toLocaleDateString()
    : 'N/A';
  const latestDate = matchingStories.length > 0
    ? new Date(Math.max(...matchingStories.map(s => new Date(s.ts).getTime()))).toLocaleDateString()
    : 'N/A';

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem', paddingBottom: '3rem' }}>
      {/* Header Banner */}
      <div
        style={{
          border: '1px solid var(--line)',
          borderRadius: '8px',
          padding: '1.25rem',
          backgroundColor: 'var(--surf)',
          borderLeft: '4px solid var(--kai)'
        }}
      >
        <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'space-between', alignItems: 'center', gap: '0.75rem' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <span
                style={{
                  backgroundColor: 'var(--kai-dim)',
                  color: '#fff',
                  fontFamily: 'var(--font-mono)',
                  fontSize: '10px',
                  fontWeight: 700,
                  padding: '2px 6px',
                  borderRadius: '3px',
                  letterSpacing: '0.05em'
                }}
              >
                MAINTENANCE // PRUNING
              </span>
              <h1 className="mono-label" style={{ color: 'var(--prim)', fontSize: '13px', margin: 0 }}>
                BULK PURGE & PRUNE CONTROL
              </h1>
            </div>
            <p style={{ margin: '0.4rem 0 0', fontSize: '0.82rem', color: 'var(--sec)' }}>
              Filter obsolete, stale, or low-ranking signals by age, topic, tier, or source. Export a deletion manifest or copy the generated CLI command.
            </p>
          </div>

          <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
            <div
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                padding: '0.35rem 0.65rem',
                borderRadius: '4px',
                border: '1px solid var(--line)',
                backgroundColor: 'var(--deep)',
                color: 'var(--prim)'
              }}
            >
              TOTAL SIGNALS: <strong style={{ color: 'var(--kai)' }}>{stories.length}</strong>
            </div>
          </div>
        </div>
      </div>

      {/* Main Grid: Left Filters, Right Preview & Actions */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '1rem' }}>
        
        {/* Filter Configuration Box */}
        <div
          style={{
            border: '1px solid var(--line)',
            borderRadius: '8px',
            padding: '1.25rem',
            backgroundColor: 'var(--surf)',
            display: 'flex',
            flexDirection: 'column',
            gap: '1.25rem'
          }}
        >
          <div style={{ borderBottom: '1px solid var(--line)', paddingBottom: '0.5rem' }}>
            <span className="mono-label" style={{ color: 'var(--kai)', fontSize: '11px' }}>
              1. FILTER CRITERIA
            </span>
          </div>

          {/* Age Window Presets */}
          <div>
            <label className="mono-label" style={{ display: 'block', marginBottom: '0.5rem', fontSize: '10px' }}>
              AGE CUTOFF (OLDER THAN):
            </label>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem', marginBottom: '0.5rem' }}>
              {(['7D', '30D', '60D', '90D', '180D', '1Y', 'ALL'] as AgePreset[]).map(preset => (
                <button
                  key={preset}
                  type="button"
                  onClick={() => {
                    setAgePreset(preset);
                    setUseCustomAge(false);
                  }}
                  className={`btn-ghost ${!useCustomAge && agePreset === preset ? 'btn-ghost-on' : ''}`}
                  style={{ fontSize: '11px', padding: '3px 8px' }}
                >
                  {preset === 'ALL' ? 'ALL TIME' : `> ${preset}`}
                </button>
              ))}
            </div>

            {/* Custom Days Toggle & Slider */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginTop: '0.5rem' }}>
              <input
                type="checkbox"
                id="custom-age-chk"
                checked={useCustomAge}
                onChange={e => setUseCustomAge(e.target.checked)}
                style={{ accentColor: 'var(--kai)', cursor: 'pointer' }}
              />
              <label htmlFor="custom-age-chk" style={{ fontSize: '0.8rem', color: 'var(--sec)', cursor: 'pointer' }}>
                Custom: Older than <strong>{customDays}</strong> days
              </label>
            </div>
            {useCustomAge && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginTop: '0.4rem' }}>
                <input
                  type="range"
                  min="1"
                  max="730"
                  value={customDays}
                  onChange={e => setCustomDays(Number(e.target.value))}
                  style={{ flex: 1, accentColor: 'var(--kai)' }}
                />
                <input
                  type="number"
                  min="1"
                  max="1000"
                  value={customDays}
                  onChange={e => setCustomDays(Math.max(1, Number(e.target.value)))}
                  style={{
                    width: '60px',
                    padding: '2px 4px',
                    backgroundColor: 'var(--deep)',
                    border: '1px solid var(--line)',
                    color: 'var(--prim)',
                    borderRadius: '4px',
                    fontFamily: 'var(--font-mono)',
                    fontSize: '11px'
                  }}
                />
              </div>
            )}
          </div>

          {/* Tier Selector */}
          <div>
            <label className="mono-label" style={{ display: 'block', marginBottom: '0.4rem', fontSize: '10px' }}>
              STORY TIER:
            </label>
            <div style={{ display: 'flex', gap: '0.35rem' }}>
              {['ALL', 'minor', 'notable', 'major'].map(tier => (
                <button
                  key={tier}
                  type="button"
                  onClick={() => setSelectedTier(tier)}
                  className={`btn-ghost ${selectedTier === tier ? 'btn-ghost-on' : ''}`}
                  style={{ fontSize: '11px', padding: '3px 8px', textTransform: 'uppercase' }}
                >
                  {tier}
                </button>
              ))}
            </div>
          </div>

          {/* Source Dropdown */}
          <div>
            <label className="mono-label" style={{ display: 'block', marginBottom: '0.4rem', fontSize: '10px' }}>
              SOURCE FILTER:
            </label>
            <select
              value={selectedSource}
              onChange={e => setSelectedSource(e.target.value)}
              style={{
                width: '100%',
                backgroundColor: 'var(--deep)',
                border: '1px solid var(--line)',
                borderRadius: '4px',
                padding: '0.45rem 0.6rem',
                color: 'var(--prim)',
                fontFamily: 'var(--font-mono)',
                fontSize: '12px'
              }}
            >
              <option value="ALL">All Sources</option>
              {topSources.map(([src, count]) => (
                <option key={src} value={src}>
                  {src} ({count})
                </option>
              ))}
            </select>
          </div>

          {/* Score Threshold Filter */}
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.35rem' }}>
              <label htmlFor="score-filter-chk" style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', cursor: 'pointer', fontSize: '11px', color: 'var(--prim)' }}>
                <input
                  type="checkbox"
                  id="score-filter-chk"
                  checked={filterByScore}
                  onChange={e => setFilterByScore(e.target.checked)}
                  style={{ accentColor: 'var(--kai)', cursor: 'pointer' }}
                />
                <span className="mono-label" style={{ fontSize: '10px' }}>SCORE THRESHOLD:</span>
              </label>
              {filterByScore && (
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--kai)' }}>
                  Score ≤ {maxScore}
                </span>
              )}
            </div>
            {filterByScore && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                <input
                  type="range"
                  min="0"
                  max="100"
                  value={maxScore}
                  onChange={e => setMaxScore(Number(e.target.value))}
                  style={{ flex: 1, accentColor: 'var(--kai)' }}
                />
                <input
                  type="number"
                  min="0"
                  max="200"
                  value={maxScore}
                  onChange={e => setMaxScore(Number(e.target.value))}
                  style={{
                    width: '50px',
                    padding: '2px 4px',
                    backgroundColor: 'var(--deep)',
                    border: '1px solid var(--line)',
                    color: 'var(--prim)',
                    borderRadius: '4px',
                    fontFamily: 'var(--font-mono)',
                    fontSize: '11px'
                  }}
                />
              </div>
            )}
          </div>

          {/* Keyword Search */}
          <div>
            <label className="mono-label" style={{ display: 'block', marginBottom: '0.4rem', fontSize: '10px' }}>
              KEYWORD / TITLE CONTAINS:
            </label>
            <input
              type="text"
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder="e.g. crypto, deprecation, test..."
              style={{
                width: '100%',
                backgroundColor: 'var(--deep)',
                border: '1px solid var(--line)',
                borderRadius: '4px',
                padding: '0.45rem 0.6rem',
                color: 'var(--prim)',
                fontFamily: 'var(--font-mono)',
                fontSize: '12px'
              }}
            />
          </div>

          {/* Tag Filter Chips */}
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.4rem' }}>
              <label className="mono-label" style={{ fontSize: '10px' }}>
                TAGS ({selectedTags.length} SELECTED):
              </label>
              {selectedTags.length > 0 && (
                <button
                  type="button"
                  onClick={() => setSelectedTags([])}
                  className="btn-ghost"
                  style={{ fontSize: '10px', padding: '1px 5px' }}
                >
                  CLEAR TAGS
                </button>
              )}
            </div>

            <input
              type="text"
              value={tagSearch}
              onChange={e => setTagSearch(e.target.value)}
              placeholder="Search tags..."
              style={{
                width: '100%',
                backgroundColor: 'var(--deep)',
                border: '1px solid var(--line)',
                borderRadius: '4px',
                padding: '0.35rem 0.5rem',
                color: 'var(--prim)',
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                marginBottom: '0.4rem'
              }}
            />

            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.3rem', maxHeight: '120px', overflowY: 'auto', paddingRight: '4px' }}>
              {filteredTagsList.map(tag => {
                const isSelected = selectedTags.includes(tag);
                return (
                  <button
                    key={tag}
                    type="button"
                    onClick={() => handleToggleTag(tag)}
                    style={{
                      fontFamily: 'var(--font-mono)',
                      fontSize: '10px',
                      padding: '2px 6px',
                      borderRadius: '3px',
                      border: isSelected ? '1px solid var(--kai)' : '1px solid var(--line)',
                      backgroundColor: isSelected ? 'var(--kai-dim)' : 'var(--deep)',
                      color: isSelected ? '#fff' : 'var(--sec)',
                      cursor: 'pointer',
                      transition: 'all 0.15s ease'
                    }}
                  >
                    #{tag}
                  </button>
                );
              })}
            </div>
          </div>

        </div>

        {/* Action & CLI Execution Deck */}
        <div
          style={{
            border: '1px solid var(--line)',
            borderRadius: '8px',
            padding: '1.25rem',
            backgroundColor: 'var(--surf)',
            display: 'flex',
            flexDirection: 'column',
            gap: '1.25rem'
          }}
        >
          <div style={{ borderBottom: '1px solid var(--line)', paddingBottom: '0.5rem' }}>
            <span className="mono-label" style={{ color: 'var(--kai)', fontSize: '11px' }}>
              2. EXECUTION & MANIFEST
            </span>
          </div>

          {/* Quick Metrics Summary */}
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(2, 1fr)',
              gap: '0.6rem',
              backgroundColor: 'var(--deep)',
              padding: '0.85rem',
              borderRadius: '6px',
              border: '1px solid var(--line)'
            }}
          >
            <div>
              <span className="mono-label" style={{ fontSize: '9px', display: 'block', color: 'var(--sec)' }}>
                SELECTED / MATCHING:
              </span>
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: '1.2rem', fontWeight: 700, color: 'var(--kai)' }}>
                {selectedCount} <span style={{ fontSize: '0.8rem', color: 'var(--sec)' }}>/ {matchingStories.length}</span>
              </span>
            </div>
            <div>
              <span className="mono-label" style={{ fontSize: '9px', display: 'block', color: 'var(--sec)' }}>
                DATE SPAN:
              </span>
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: '0.75rem', color: 'var(--prim)' }}>
                {earliestDate} → {latestDate}
              </span>
            </div>
          </div>

          {/* Manifest Export Buttons */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
            <button
              type="button"
              onClick={handleCopyManifest}
              disabled={selectedCount === 0}
              className="btn-ghost"
              style={{
                width: '100%',
                padding: '0.6rem',
                justifyContent: 'center',
                backgroundColor: isCopiedJson ? 'var(--kai)' : 'var(--deep)',
                borderColor: isCopiedJson ? 'var(--kai)' : 'var(--line-light)',
                color: isCopiedJson ? '#fff' : 'var(--prim)',
                fontWeight: 600
              }}
            >
              {isCopiedJson ? '✓ SLUGS COPIED TO CLIPBOARD' : `📋 COPY SLUGS MANIFEST (${selectedCount} SLUGS)`}
            </button>

            <button
              type="button"
              onClick={handleDownloadManifest}
              disabled={selectedCount === 0}
              className="btn-ghost"
              style={{
                width: '100%',
                padding: '0.6rem',
                justifyContent: 'center',
                backgroundColor: 'var(--deep)',
                borderColor: 'var(--line)'
              }}
            >
              💾 DOWNLOAD PURGE-MANIFEST.JSON
            </button>
          </div>

          {/* Generated CLI Command Section */}
          <div style={{ borderTop: '1px solid var(--line)', paddingTop: '1rem' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem' }}>
              <span className="mono-label" style={{ fontSize: '10px' }}>
                CLI RUN COMMAND:
              </span>
              <label style={{ display: 'flex', alignItems: 'center', gap: '0.35rem', cursor: 'pointer', fontSize: '11px', color: 'var(--sec)' }}>
                <input
                  type="checkbox"
                  checked={includeDryRunInCli}
                  onChange={e => setIncludeDryRunInCli(e.target.checked)}
                  style={{ accentColor: 'var(--kai)', cursor: 'pointer' }}
                />
                Dry Run (--dry)
              </label>
            </div>

            {/* Direct Flags Command */}
            <div
              style={{
                position: 'relative',
                backgroundColor: 'var(--deep)',
                border: '1px solid var(--line)',
                borderRadius: '4px',
                padding: '0.75rem',
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                color: 'var(--kai)',
                wordBreak: 'break-all',
                marginBottom: '0.5rem'
              }}
            >
              <code>{cliCommand}</code>
              <button
                type="button"
                onClick={() => handleCopyCli(cliCommand)}
                className="btn-ghost"
                style={{
                  position: 'absolute',
                  top: '4px',
                  right: '4px',
                  fontSize: '9px',
                  padding: '2px 6px',
                  backgroundColor: isCopiedCli ? 'var(--kai)' : 'var(--surf)'
                }}
              >
                {isCopiedCli ? 'COPIED' : 'COPY'}
              </button>
            </div>

            {/* Manifest Mode Command */}
            <div
              style={{
                backgroundColor: 'var(--deep)',
                border: '1px solid var(--line)',
                borderRadius: '4px',
                padding: '0.6rem 0.75rem',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                fontFamily: 'var(--font-mono)',
                fontSize: '11px'
              }}
            >
              <code style={{ color: 'var(--sec)' }}>
                ./agent/bin/airfoil purge --slug-file purge-manifest.json
              </code>
              <button
                type="button"
                onClick={() => handleCopyCli('./agent/bin/airfoil purge --slug-file purge-manifest.json')}
                className="btn-ghost"
                style={{ fontSize: '9px', padding: '2px 6px' }}
              >
                COPY
              </button>
            </div>
          </div>

          {/* Safety Notice */}
          <div
            style={{
              padding: '0.75rem',
              backgroundColor: 'rgba(255, 59, 59, 0.05)',
              border: '1px solid var(--kai-glow)',
              borderRadius: '4px',
              fontSize: '0.75rem',
              color: 'var(--sec)',
              lineHeight: 1.4
            }}
          >
            <strong style={{ color: 'var(--kai)' }}>Note:</strong> Airfoil is a static Git-backed intelligence site. Purging removes records from <code>data/stories.json</code>, updates <code>index.json</code>, and deletes orphaned markdown files on disk.
          </div>

        </div>

      </div>

      {/* Preview & Selection Table */}
      <div
        style={{
          border: '1px solid var(--line)',
          borderRadius: '8px',
          backgroundColor: 'var(--surf)',
          overflow: 'hidden'
        }}
      >
        {/* Table Controls Header */}
        <div
          style={{
            padding: '0.85rem 1.25rem',
            borderBottom: '1px solid var(--line)',
            display: 'flex',
            flexWrap: 'wrap',
            justifyContent: 'space-between',
            alignItems: 'center',
            gap: '0.75rem',
            backgroundColor: 'var(--deep)'
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <span className="mono-label" style={{ fontSize: '11px', color: 'var(--prim)' }}>
              MATCHING SIGNALS ({matchingStories.length})
            </span>
            <div style={{ display: 'flex', gap: '0.35rem' }}>
              <button
                type="button"
                onClick={handleSelectAllMatching}
                className="btn-ghost"
                style={{ fontSize: '10px', padding: '2px 7px' }}
              >
                SELECT ALL
              </button>
              <button
                type="button"
                onClick={handleDeselectAll}
                className="btn-ghost"
                style={{ fontSize: '10px', padding: '2px 7px' }}
              >
                DESELECT ALL
              </button>
              <button
                type="button"
                onClick={handleInvertSelection}
                className="btn-ghost"
                style={{ fontSize: '10px', padding: '2px 7px' }}
              >
                INVERT
              </button>
            </div>
          </div>

          {/* Sort selector */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
            <span className="mono-label" style={{ fontSize: '9px' }}>SORT:</span>
            <select
              value={sortBy}
              onChange={e => setSortBy(e.target.value as any)}
              style={{
                backgroundColor: 'var(--surf)',
                border: '1px solid var(--line)',
                color: 'var(--prim)',
                borderRadius: '4px',
                padding: '2px 6px',
                fontFamily: 'var(--font-mono)',
                fontSize: '11px'
              }}
            >
              <option value="oldest">Oldest First (Default)</option>
              <option value="newest">Newest First</option>
              <option value="score_asc">Score (Low → High)</option>
              <option value="score_desc">Score (High → Low)</option>
            </select>
          </div>
        </div>

        {/* Stories Rows List */}
        {matchingStories.length > 0 ? (
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            {matchingStories.map(story => {
              const isSelected = selectedSlugs[story.slug] !== undefined ? selectedSlugs[story.slug] : true;
              const dateStr = new Date(story.ts).toISOString().slice(0, 10);
              const ageDays = Math.round((Date.now() - new Date(story.ts).getTime()) / (1000 * 60 * 60 * 24));

              return (
                <div
                  key={story.id}
                  onClick={() => handleToggleStory(story.slug)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.85rem',
                    padding: '0.75rem 1.25rem',
                    borderBottom: '1px solid var(--line)',
                    backgroundColor: isSelected ? 'rgba(255, 59, 59, 0.04)' : 'transparent',
                    cursor: 'pointer',
                    transition: 'background-color 0.15s ease'
                  }}
                >
                  {/* Row Checkbox */}
                  <input
                    type="checkbox"
                    checked={isSelected}
                    onChange={() => handleToggleStory(story.slug)}
                    onClick={e => e.stopPropagation()}
                    style={{ accentColor: 'var(--kai)', cursor: 'pointer', width: '15px', height: '15px' }}
                  />

                  {/* Date & Age */}
                  <div style={{ minWidth: '100px', flexShrink: 0, fontFamily: 'var(--font-mono)', fontSize: '11px' }}>
                    <span style={{ color: 'var(--prim)', display: 'block' }}>{dateStr}</span>
                    <span style={{ color: 'var(--kai-dim)', fontSize: '10px' }}>{ageDays}D AGO</span>
                  </div>

                  {/* Score & Tier Badge */}
                  <div
                    style={{
                      padding: '2px 6px',
                      borderRadius: '3px',
                      border: '1px solid var(--line)',
                      backgroundColor: 'var(--deep)',
                      fontFamily: 'var(--font-mono)',
                      fontSize: '11px',
                      color: story.score >= 50 ? 'var(--kai)' : 'var(--sec)',
                      minWidth: '38px',
                      textAlign: 'center',
                      flexShrink: 0
                    }}
                  >
                    {story.score}
                  </div>

                  {/* Title & Excerpt */}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap' }}>
                      <span
                        style={{
                          fontWeight: 500,
                          fontSize: '0.9rem',
                          color: isSelected ? 'var(--prim)' : 'var(--sec)',
                          textDecoration: isSelected ? 'line-through' : 'none'
                        }}
                      >
                        {story.title}
                      </span>
                    </div>

                    <div style={{ display: 'flex', gap: '0.4rem', marginTop: '0.25rem', flexWrap: 'wrap', alignItems: 'center' }}>
                      {story.tags.slice(0, 3).map(tag => (
                        <span
                          key={tag}
                          style={{
                            fontFamily: 'var(--font-mono)',
                            fontSize: '9px',
                            color: 'var(--sec)',
                            backgroundColor: 'var(--deep)',
                            padding: '1px 4px',
                            borderRadius: '2px'
                          }}
                        >
                          #{tag}
                        </span>
                      ))}

                      <span style={{ fontSize: '10px', color: 'var(--sec)', marginLeft: '0.25rem' }}>
                        {story.sources.length} sources ({story.sources.map(s => s.name).join(', ')})
                      </span>
                    </div>
                  </div>

                  {/* View Action */}
                  <button
                    type="button"
                    onClick={e => {
                      e.stopPropagation();
                      onOpenStory(story);
                    }}
                    className="btn-ghost"
                    style={{ fontSize: '10px', padding: '2px 6px', flexShrink: 0 }}
                  >
                    VIEW →
                  </button>
                </div>
              );
            })}
          </div>
        ) : (
          <div style={{ padding: '3rem 1.5rem', textAlign: 'center' }}>
            <p className="mono-label" style={{ color: 'var(--sec)', marginBottom: '0.5rem' }}>
              NO STORIES MATCH THE PURGE CRITERIA
            </p>
            <p style={{ fontSize: '0.8rem', color: 'var(--sec)' }}>
              Try adjusting the age cutoff slider or clearing tag and source filters.
            </p>
          </div>
        )}
      </div>

    </div>
  );
};
