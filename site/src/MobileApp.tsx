import React, { useState } from 'react';
import { Routes, Route, useNavigate, useParams } from 'react-router-dom';
import { MobileHeader } from './components/mobile/MobileHeader';
import { MobileTabBar } from './components/mobile/MobileTabBar';
import { MobileFilterDrawer } from './components/mobile/MobileFilterDrawer';
import { MobileFeedPage } from './pages/mobile/MobileFeedPage';
import { MobileDigestPage } from './pages/mobile/MobileDigestPage';
import { MobileTopPage } from './pages/mobile/MobileTopPage';
import { MobileShipPage } from './pages/mobile/MobileShipPage';
import { MobileSearchPage } from './pages/mobile/MobileSearchPage';
import { MobileStoryDetail } from './components/mobile/MobileStoryDetail';
import { PurgePage } from './pages/PurgePage';
import type { Story, DateWindow, SortOrder, FilterState } from './types/story';
import type { Theme } from './hooks/useTheme';
import './MobileApp.css';

interface MobileAppProps {
  stories: Story[];
  filteredStories: Story[];
  filters: FilterState;
  allTags: string[];
  topSources: [string, number][];
  topStory: Story | null;
  totalSourcesCount: number;
  setDate: (date: DateWindow) => void;
  setTag: (tag: string | null) => void;
  setSource: (source: string | null) => void;
  setQuery: (query: string) => void;
  setSortBy: (sortBy: SortOrder) => void;
  resetFilters: () => void;
  theme: Theme;
  toggleTheme: () => void;
}

// Wrapper for Story Detail Route
const MobileStoryRoute: React.FC<{
  stories: Story[];
  onSelectTag: (tag: string) => void;
}> = ({ stories, onSelectTag }) => {
  const { slug } = useParams<{ slug: string }>();
  const navigate = useNavigate();
  const story = stories.find(s => s.slug === slug || s.id === slug);

  if (!story) {
    return (
      <div
        style={{
          border: '1px solid var(--line)',
          borderRadius: '8px',
          padding: '2rem 1rem',
          textAlign: 'center',
          backgroundColor: 'var(--surf)'
        }}
      >
        <p className="mono-label" style={{ marginBottom: '1rem', color: 'var(--kai)' }}>
          SIGNAL NOT FOUND
        </p>
        <button
          type="button"
          onClick={() => navigate('/feed')}
          className="btn-ghost"
        >
          ← RETURN TO FEED
        </button>
      </div>
    );
  }

  return (
    <MobileStoryDetail
      story={story}
      onBack={() => navigate(-1)}
      onSelectTag={tag => {
        onSelectTag(tag);
        navigate('/feed');
      }}
    />
  );
};

export const MobileApp: React.FC<MobileAppProps> = ({
  stories,
  filteredStories,
  filters,
  allTags,
  topSources,
  topStory,
  setDate,
  setTag,
  setSource,
  setQuery,
  setSortBy,
  resetFilters,
  theme,
  toggleTheme
}) => {
  const navigate = useNavigate();
  const [isFilterDrawerOpen, setIsFilterDrawerOpen] = useState(false);

  const handleOpenStory = (story: Story) => {
    navigate(`/story/${story.slug}`);
  };

  const handleSelectTag = (tag: string) => {
    setTag(tag);
  };

  const handleSelectSource = (source: string) => {
    setSource(source);
  };

  // Count how many active filters there are
  const activeFilterCount =
    (filters.date !== 'ALL' ? 1 : 0) +
    (filters.tag !== null ? 1 : 0) +
    (filters.source !== null ? 1 : 0) +
    (filters.sortBy !== 'newest' ? 1 : 0) +
    (filters.query ? 1 : 0);

  return (
    <div className="mobile-app-shell">
      {/* Slim Fixed Mobile Header */}
      <MobileHeader
        theme={theme}
        onToggleTheme={toggleTheme}
        onOpenFilters={() => setIsFilterDrawerOpen(true)}
        activeFilterCount={activeFilterCount}
      />

      {/* Main Content Area */}
      <main className="mobile-content-wrap">
        <Routes>
          <Route
            path="/"
            element={
              <MobileFeedPage
                stories={filteredStories}
                totalStoriesCount={stories.length}
                filters={filters}
                onClearTag={() => setTag(null)}
                onClearSource={() => setSource(null)}
                onClearDate={() => setDate('ALL')}
                onClearQuery={() => setQuery('')}
                onSelectDate={setDate}
                onSelectSortBy={setSortBy}
                selectedTag={filters.tag}
                onSelectTag={handleSelectTag}
                onOpenStory={handleOpenStory}
                onResetFilters={resetFilters}
                topStory={topStory}
              />
            }
          />
          <Route
            path="/feed"
            element={
              <MobileFeedPage
                stories={filteredStories}
                totalStoriesCount={stories.length}
                filters={filters}
                onClearTag={() => setTag(null)}
                onClearSource={() => setSource(null)}
                onClearDate={() => setDate('ALL')}
                onClearQuery={() => setQuery('')}
                onSelectDate={setDate}
                onSelectSortBy={setSortBy}
                selectedTag={filters.tag}
                onSelectTag={handleSelectTag}
                onOpenStory={handleOpenStory}
                onResetFilters={resetFilters}
                topStory={topStory}
              />
            }
          />
          <Route
            path="/digest"
            element={
              <MobileDigestPage
                stories={filteredStories}
                onOpenStory={handleOpenStory}
                selectedTag={filters.tag}
                onSelectTag={handleSelectTag}
              />
            }
          />
          <Route
            path="/top"
            element={
              <MobileTopPage
                stories={filteredStories}
                onOpenStory={handleOpenStory}
                selectedTag={filters.tag}
                onSelectTag={handleSelectTag}
                dateWindow={filters.date}
                onSelectDate={setDate}
                onResetFilters={resetFilters}
              />
            }
          />
          <Route
            path="/ship"
            element={
              <MobileShipPage
                stories={filteredStories}
                totalStoriesCount={stories.length}
                filters={filters}
                onClearTag={() => setTag(null)}
                onClearSource={() => setSource(null)}
                onClearDate={() => setDate('ALL')}
                onClearQuery={() => setQuery('')}
                onSelectDate={setDate}
                onSelectSortBy={setSortBy}
                onOpenStory={handleOpenStory}
                selectedTag={filters.tag}
                onSelectTag={handleSelectTag}
                onResetFilters={resetFilters}
              />
            }
          />
          <Route
            path="/search"
            element={
              <MobileSearchPage
                stories={filteredStories}
                query={filters.query}
                onQueryChange={setQuery}
                onOpenStory={handleOpenStory}
                selectedTag={filters.tag}
                onSelectTag={handleSelectTag}
                tags={allTags}
              />
            }
          />
          <Route
            path="/purge"
            element={
              <div style={{ padding: '0.5rem 0.25rem' }}>
                <PurgePage
                  stories={stories}
                  allTags={allTags}
                  topSources={topSources}
                  onOpenStory={handleOpenStory}
                />
              </div>
            }
          />
          <Route
            path="/story/:slug"
            element={
              <MobileStoryRoute
                stories={stories}
                onSelectTag={handleSelectTag}
              />
            }
          />
        </Routes>
      </main>

      {/* Fixed Bottom Tab Bar Navigation */}
      <MobileTabBar />

      {/* Slide-up Filter Bottom Sheet Drawer */}
      <MobileFilterDrawer
        isOpen={isFilterDrawerOpen}
        onClose={() => setIsFilterDrawerOpen(false)}
        dateWindow={filters.date}
        onSelectDate={setDate}
        sortBy={filters.sortBy}
        onSelectSortBy={setSortBy}
        tags={allTags}
        selectedTag={filters.tag}
        onSelectTag={handleSelectTag}
        topSources={topSources}
        selectedSource={filters.source}
        onSelectSource={handleSelectSource}
        onResetFilters={resetFilters}
      />
    </div>
  );
};
