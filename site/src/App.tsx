import { useState } from 'react';
import { Routes, Route, useNavigate, useLocation } from 'react-router-dom';
import { useAirfoilSignals } from './hooks/useAirfoilSignals';
import { useTheme } from './hooks/useTheme';
import { Header } from './components/Header';
import { LeftRail } from './components/LeftRail';
import { RightRail } from './components/RightRail';
import { Footer } from './components/Footer';
import { MobileOverlay } from './components/MobileOverlay';
import { HomePage } from './pages/HomePage';
import { FeedPage } from './pages/FeedPage';
import { DigestPage } from './pages/DigestPage';
import { TopPage } from './pages/TopPage';
import { ShipPage } from './pages/ShipPage';
import { StoryPage } from './pages/StoryPage';
import { SearchPage } from './pages/SearchPage';
import PixelBlast from './components/PixelBlast/PixelBlast';
import type { Story, DateWindow } from './types/story';

export function App() {
  const navigate = useNavigate();
  const location = useLocation();
  const { theme, toggleTheme } = useTheme();

  const {
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
  } = useAirfoilSignals();

  const [isSearchOpen, setIsSearchOpen] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  const handleOpenStory = (story: Story) => {
    navigate(`/story/${story.slug}`);
  };

  const handleSelectTag = (tag: string) => {
    setTag(tag);
    if (location.pathname.startsWith('/story/') || location.pathname === '/') {
      navigate('/feed');
    }
  };

  const handleSelectSource = (source: string) => {
    setSource(source);
    if (location.pathname.startsWith('/story/') || location.pathname === '/') {
      navigate('/feed');
    }
  };

  const handleSelectDate = (date: DateWindow) => {
    setDate(date);
    if (location.pathname.startsWith('/story/') || location.pathname === '/') {
      navigate('/feed');
    }
  };

  const isStoryView = location.pathname.startsWith('/story/');
  const isLandingPage = location.pathname === '/';

  return (
    <>
      {/* PixelBlast fixed full-page background — sitewide, z-index 0 */}
      <div
        style={{
          position: 'fixed',
          inset: 0,
          zIndex: 0,
          backgroundColor: 'var(--deep)',
          pointerEvents: 'none',
          transition: 'background-color 0.2s ease'
        }}
      >
        <PixelBlast
          color={theme === 'light' ? '#e62e2e' : '#ff3b3b'}
          variant="circle"
          pixelSize={2.5}
          patternScale={1.8}
          patternDensity={theme === 'light' ? 0.72 : 0.82}
          speed={0.38}
          enableRipples={true}
          rippleSpeed={0.28}
          rippleThickness={0.09}
          rippleIntensityScale={1.4}
          edgeFade={0.0}
          transparent={true}
          pixelSizeJitter={0.6}
        />
      </div>

      {/* All site content — z-index 1, above PixelBlast */}
      <div style={{ position: 'relative', zIndex: 1, minHeight: '100vh', display: 'flex', flexDirection: 'column', transition: 'color 0.2s ease' }}>
      {/* Fixed Telemetry Header */}
      <Header
        searchQuery={filters.query}
        onSearchChange={query => {
          setQuery(query);
          if (location.pathname === '/') navigate('/feed');
        }}
        isSearchOpen={isSearchOpen}
        onToggleSearch={() => setIsSearchOpen(prev => !prev)}
        onOpenMobileMenu={() => setIsMobileMenuOpen(true)}
        theme={theme}
        onToggleTheme={toggleTheme}
      />

      {/* RENDER DEDICATED SEPARATE FULL-WIDTH HOME PAGE ON '/' */}
      {isLandingPage ? (
        <div className="home-bg" style={{ paddingTop: 'var(--header-height)', minHeight: 'calc(100vh - var(--header-height))', width: '100%' }}>
          <div className="app-container" style={{ maxWidth: '1240px', margin: '0 auto', padding: '2rem 1rem' }}>
            <HomePage
              stories={stories}
              topStory={topStory}
              totalSourcesCount={totalSourcesCount}
              onOpenStory={handleOpenStory}
              selectedTag={filters.tag}
              onSelectTag={handleSelectTag}
            />
          </div>
        </div>
      ) : (
        /* RENDER 3-COLUMN FLIGHT DECK GRID FOR APP VIEWS (/feed, /digest, /top, /ship, /story/:slug, /search) */
        <div className="app-glass-wrap">
          <div className="main-grid">
          {/* Left Rail — Control Surfaces */}
          <div className="left-rail-container">
            <LeftRail
              dateWindow={filters.date}
              onSelectDate={handleSelectDate}
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

          {/* Center Maindeck — Signal Feeds & Analysis */}
          <main className="center-feed-container">
            <Routes>
              <Route
                path="/feed"
                element={
                  <FeedPage
                    stories={filteredStories}
                    totalStoriesCount={stories.length}
                    filters={filters}
                    onClearTag={() => setTag(null)}
                    onClearSource={() => setSource(null)}
                    onClearDate={() => setDate('ALL')}
                    onClearQuery={() => setQuery('')}
                    onSelectSortBy={setSortBy}
                    selectedTag={filters.tag}
                    onSelectTag={handleSelectTag}
                    onOpenStory={handleOpenStory}
                    onResetFilters={resetFilters}
                  />
                }
              />
              <Route
                path="/digest"
                element={
                  <DigestPage
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
                  <TopPage
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
                  <ShipPage
                    stories={filteredStories}
                    totalStoriesCount={stories.length}
                    filters={filters}
                    onClearTag={() => setTag(null)}
                    onClearSource={() => setSource(null)}
                    onClearDate={() => setDate('ALL')}
                    onClearQuery={() => setQuery('')}
                    onSelectSortBy={setSortBy}
                    onOpenStory={handleOpenStory}
                    selectedTag={filters.tag}
                    onSelectTag={handleSelectTag}
                    onResetFilters={resetFilters}
                  />
                }
              />
              <Route
                path="/story/:slug"
                element={
                  <StoryPage
                    stories={stories}
                    onSelectTag={handleSelectTag}
                  />
                }
              />
              <Route
                path="/search"
                element={
                  <SearchPage
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
            </Routes>
          </main>

          {/* Right Rail — Aerodynamics & Telemetry */}
          <div className="right-rail-container">
            <RightRail
              topStory={topStory}
              onOpenStory={handleOpenStory}
            />
          </div>
          </div>
        </div>
      )}

      {/* Dynamic Telemetry Footer */}
      <Footer
        currentCount={isStoryView ? 1 : filteredStories.length}
        totalCount={stories.length}
        sourcesCount={totalSourcesCount}
      />

      {/* Mobile Drawer Overlay */}
      <MobileOverlay
        isOpen={isMobileMenuOpen}
        onClose={() => setIsMobileMenuOpen(false)}
        dateWindow={filters.date}
        onSelectDate={handleSelectDate}
        sortBy={filters.sortBy}
        onSelectSortBy={setSortBy}
        tags={allTags}
        selectedTag={filters.tag}
        onSelectTag={handleSelectTag}
        topSources={topSources}
        selectedSource={filters.source}
        onSelectSource={handleSelectSource}
        topStory={topStory}
        onOpenStory={handleOpenStory}
        onResetFilters={resetFilters}
      />
    </div>
    </>
  );
}

export default App;
