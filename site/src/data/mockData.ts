import type { Story } from '../types/story';
import rawStories from '../../../data/stories.json';

/**
 * Airfoil Live Data Stream
 * Directly imported from root `/data/stories.json` database.
 * Updating `/data/stories.json` or running the ingest agent automatically updates the UI.
 */
export const MOCK_STORIES: Story[] = (rawStories as Story[]).map(story => ({
  ...story,
  // Ensure builder_relevant is explicitly a boolean
  builder_relevant: story.builder_relevant !== false
}));
