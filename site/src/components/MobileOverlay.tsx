import React from 'react';
import { LeftRail } from './LeftRail';
import { RightRail } from './RightRail';
import type { Story, DateWindow, SortOrder } from '../types/story';

interface MobileOverlayProps {
  isOpen: boolean;
  onClose: () => void;
  dateWindow: DateWindow;
  onSelectDate: (date: DateWindow) => void;
  sortBy?: SortOrder;
  onSelectSortBy?: (sort: SortOrder) => void;
  tags: string[];
  selectedTag: string | null;
  onSelectTag: (tag: string) => void;
  topSources: [string, number][];
  selectedSource: string | null;
  onSelectSource: (source: string) => void;
  topStory: Story | null;
  onOpenStory: (story: Story) => void;
  onResetFilters: () => void;
}

export const MobileOverlay: React.FC<MobileOverlayProps> = ({
  isOpen,
  onClose,
  dateWindow,
  onSelectDate,
  sortBy = 'newest',
  onSelectSortBy,
  tags,
  selectedTag,
  onSelectTag,
  topSources,
  selectedSource,
  onSelectSource,
  topStory,
  onOpenStory,
  onResetFilters
}) => {
  if (!isOpen) return null;

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 60,
        backgroundColor: 'rgba(10, 10, 10, 0.98)',
        backdropFilter: 'blur(12px)',
        overflowY: 'auto',
        padding: '1.25rem 1rem 3rem'
      }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: '1.5rem',
          borderBottom: '1px solid var(--line)',
          paddingBottom: '0.75rem'
        }}
      >
        <span className="mono-label" style={{ color: 'var(--kai)' }}>
          // CONTROL SURFACES
        </span>
        <button
          type="button"
          onClick={onClose}
          className="btn-ghost"
          style={{ borderColor: 'var(--line-light)' }}
        >
          ✕ CLOSE
        </button>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
        <LeftRail
          dateWindow={dateWindow}
          onSelectDate={d => {
            onSelectDate(d);
            onClose();
          }}
          sortBy={sortBy}
          onSelectSortBy={s => {
            onSelectSortBy?.(s);
            onClose();
          }}
          tags={tags}
          selectedTag={selectedTag}
          onSelectTag={t => {
            onSelectTag(t);
            onClose();
          }}
          topSources={topSources}
          selectedSource={selectedSource}
          onSelectSource={s => {
            onSelectSource(s);
            onClose();
          }}
          onResetFilters={() => {
            onResetFilters();
            onClose();
          }}
        />

        <RightRail
          topStory={topStory}
          onOpenStory={s => {
            onOpenStory(s);
            onClose();
          }}
        />
      </div>
    </div>
  );
};
