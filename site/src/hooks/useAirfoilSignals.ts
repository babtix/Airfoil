import { useState, useMemo } from 'react';
import { MOCK_STORIES } from '../data/mockData';
import type { Story, DateWindow, FilterState, SortOrder } from '../types/story';

export function getStoryAgeHours(story: Story): number {
  const storyTime = new Date(story.ts).getTime();
  return (Date.now() - storyTime) / 3600000;
}

export function formatStoryAge(story: Story): string {
  const hours = getStoryAgeHours(story);
  if (hours < 1) {
    const mins = Math.max(1, Math.round(hours * 60));
    return `${mins}M AGO`;
  }
  if (hours < 24) {
    return `${Math.round(hours)}H AGO`;
  }
  const days = Math.round(hours / 24);
  return `${days}D AGO`;
}

export function useAirfoilSignals(initialStories: Story[] = MOCK_STORIES) {
  const [stories] = useState<Story[]>(initialStories);
  const [filters, setFilters] = useState<FilterState>({
    date: 'ALL',
    tag: null,
    source: null,
    query: '',
    sortBy: 'newest'
  });

  const allTags = useMemo(() => {
    const tagSet = new Set<string>();
    stories.forEach(s => s.tags.forEach(t => tagSet.add(t)));
    return Array.from(tagSet).sort();
  }, [stories]);

  const sourceCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    stories.forEach(s => {
      s.sources.forEach(src => {
        counts[src.name] = (counts[src.name] || 0) + 1;
      });
    });
    return counts;
  }, [stories]);

  const topSources = useMemo(() => {
    return Object.entries(sourceCounts)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 6);
  }, [sourceCounts]);

  const totalSourcesCount = useMemo(() => {
    return Object.keys(sourceCounts).length;
  }, [sourceCounts]);

  const topStory = useMemo(() => {
    if (!stories.length) return null;
    return [...stories].sort((a, b) => b.score - a.score)[0];
  }, [stories]);

  const filteredStories = useMemo(() => {
    return stories
      .filter(s => {
        // Date filter
        const ageHours = getStoryAgeHours(s);
        if (filters.date === '24H' && ageHours > 24) return false;
        if (filters.date === '7D' && ageHours > 168) return false;

        // Tag filter
        if (filters.tag && !s.tags.includes(filters.tag)) return false;

        // Source filter
        if (filters.source && !s.sources.some(src => src.name === filters.source)) return false;

        // Search Query filter
        if (filters.query.trim()) {
          const q = filters.query.toLowerCase();
          const target = `${s.title} ${s.summary} ${s.tags.join(' ')} ${s.body.join(' ')}`.toLowerCase();
          if (!target.includes(q)) return false;
        }

        return true;
      })
      .sort((a, b) => {
        if (filters.sortBy === 'score') {
          return b.score - a.score;
        }
        if (filters.sortBy === 'cluster') {
          return b.cluster - a.cluster;
        }
        // Default: newest (latest timestamp first)
        return new Date(b.ts).getTime() - new Date(a.ts).getTime();
      });
  }, [stories, filters]);

  const setDate = (date: DateWindow) => setFilters(prev => ({ ...prev, date }));
  const setTag = (tag: string | null) => setFilters(prev => ({ ...prev, tag: prev.tag === tag ? null : tag }));
  const setSource = (source: string | null) => setFilters(prev => ({ ...prev, source: prev.source === source ? null : source }));
  const setQuery = (query: string) => setFilters(prev => ({ ...prev, query }));
  const setSortBy = (sortBy: SortOrder) => setFilters(prev => ({ ...prev, sortBy }));
  const resetFilters = () => setFilters({ date: 'ALL', tag: null, source: null, query: '', sortBy: 'newest' });

  return {
    stories,
    filteredStories,
    filters,
    allTags,
    topSources,
    topStory,
    totalSourcesCount,
    setDate,
    setTag,
    setSource,
    setQuery,
    setSortBy,
    resetFilters
  };
}
