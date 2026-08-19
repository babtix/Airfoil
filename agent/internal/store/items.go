package store

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/papitsho/airfoil/internal/model"
)

// ItemsPath is the file holding one day of raw items.
func ItemsPath(itemsDir string, day time.Time) string {
	return filepath.Join(itemsDir, day.UTC().Format(time.DateOnly)+".json")
}

// LoadItems reads the items for a single day. A missing file yields no items.
func LoadItems(itemsDir string, day time.Time) ([]model.Item, error) {
	return ReadJSONOr(ItemsPath(itemsDir, day), []model.Item(nil))
}

// LoadItemsSince reads every day's items from the last n days, newest first.
//
// Clustering runs over a rolling window, so it needs yesterday's items as well
// as today's — a story that breaks at 23:50 must still cluster at 00:10.
func LoadItemsSince(itemsDir string, now time.Time, days int) ([]model.Item, error) {
	var all []model.Item
	seen := make(map[string]bool)

	for i := range days {
		day := now.AddDate(0, 0, -i)
		items, err := LoadItems(itemsDir, day)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if seen[item.ID] {
				continue
			}
			seen[item.ID] = true
			all = append(all, item)
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		return all[i].PublishedAt.After(all[j].PublishedAt)
	})
	return all, nil
}

// AppendItems merges new items into a day's file and writes it back.
//
// The pipeline runs several times a day, so this must be additive. Items
// already present are left alone, which is what keeps a repeat run from
// duplicating a day's contents (R7).
func AppendItems(itemsDir string, day time.Time, items []model.Item) (added int, err error) {
	existing, err := LoadItems(itemsDir, day)
	if err != nil {
		return 0, err
	}

	seen := make(map[string]bool, len(existing))
	for _, item := range existing {
		seen[item.ID] = true
	}

	merged := existing
	for _, item := range items {
		if seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		merged = append(merged, item)
		added++
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].PublishedAt.After(merged[j].PublishedAt)
	})

	if err := WriteJSON(ItemsPath(itemsDir, day), merged); err != nil {
		return 0, fmt.Errorf("store: write items for %s: %w", day.Format(time.DateOnly), err)
	}
	return added, nil
}

// LoadState reads state.json, returning empty state if it does not exist.
func LoadState(path string) (*model.State, error) {
	state, err := ReadJSONOr(path, model.NewState())
	if err != nil {
		return nil, err
	}
	if state == nil {
		return model.NewState(), nil
	}
	if state.SeenURLs == nil {
		state.SeenURLs = map[string]string{}
	}
	if state.Cursors == nil {
		state.Cursors = map[string]string{}
	}
	return state, nil
}

// SaveState writes state.json atomically.
func SaveState(path string, state *model.State) error {
	return WriteJSON(path, state)
}
