// Package purge provides filtering logic for bulk-deleting stale or unwanted
// stories from the authoritative record and index.
package purge

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/model"
)

// Options specifies the filter criteria for matching stories to purge.
// All configured criteria are combined with AND logic.
type Options struct {
	// Before matches stories published strictly before this cutoff time.
	Before time.Time

	// Tags matches stories containing ANY of the specified tags.
	Tags []string

	// Sources matches stories containing ANY of the specified source names (case-insensitive).
	Sources []string

	// Tier matches stories with this exact tier (e.g. "minor", "notable", "major").
	Tier string

	// MaxScore matches stories with Score <= MaxScore.
	MaxScore *int

	// Slugs matches stories whose slug is in this set.
	Slugs []string

	// Query matches stories whose Title, Summary, or Tags contain this substring (case-insensitive).
	Query string
}

// IsEmpty reports whether no filter criteria have been specified.
func (o Options) IsEmpty() bool {
	return o.Before.IsZero() &&
		len(o.Tags) == 0 &&
		len(o.Sources) == 0 &&
		o.Tier == "" &&
		o.MaxScore == nil &&
		len(o.Slugs) == 0 &&
		strings.TrimSpace(o.Query) == ""
}

// Result contains the partition of kept and removed stories.
type Result struct {
	Kept    []model.Story
	Removed []model.Story
}

// Filter partitions a list of stories into Kept and Removed based on the options.
// If opts.IsEmpty() is true, all stories are kept and none are removed.
func Filter(stories []model.Story, opts Options) Result {
	if opts.IsEmpty() {
		return Result{
			Kept:    append([]model.Story(nil), stories...),
			Removed: nil,
		}
	}

	slugMap := make(map[string]bool, len(opts.Slugs))
	for _, slug := range opts.Slugs {
		slugMap[strings.TrimSpace(slug)] = true
	}

	tagMap := make(map[string]bool, len(opts.Tags))
	for _, tag := range opts.Tags {
		tagMap[strings.ToLower(strings.TrimSpace(tag))] = true
	}

	sourceMap := make(map[string]bool, len(opts.Sources))
	for _, src := range opts.Sources {
		sourceMap[strings.ToLower(strings.TrimSpace(src))] = true
	}

	query := strings.ToLower(strings.TrimSpace(opts.Query))
	targetTier := strings.ToLower(strings.TrimSpace(opts.Tier))

	var kept []model.Story
	var removed []model.Story

	for _, s := range stories {
		if matches(s, opts, slugMap, tagMap, sourceMap, targetTier, query) {
			removed = append(removed, s)
		} else {
			kept = append(kept, s)
		}
	}

	return Result{
		Kept:    kept,
		Removed: removed,
	}
}

func matches(
	s model.Story,
	opts Options,
	slugMap map[string]bool,
	tagMap map[string]bool,
	sourceMap map[string]bool,
	targetTier string,
	query string,
) bool {
	// Slugs filter (if provided, story must match one of the slugs)
	if len(slugMap) > 0 {
		if !slugMap[s.Slug] {
			return false
		}
	}

	// Before filter
	if !opts.Before.IsZero() {
		if !s.Date.Before(opts.Before) {
			return false
		}
	}

	// Tier filter
	if targetTier != "" {
		if strings.ToLower(s.Tier) != targetTier {
			return false
		}
	}

	// MaxScore filter
	if opts.MaxScore != nil {
		if s.Score > *opts.MaxScore {
			return false
		}
	}

	// Tags filter (matches if story contains ANY of the requested tags)
	if len(tagMap) > 0 {
		hasTag := false
		for _, t := range s.Tags {
			if tagMap[strings.ToLower(t)] {
				hasTag = true
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	// Sources filter (matches if story contains ANY of the requested source names)
	if len(sourceMap) > 0 {
		hasSource := false
		for _, src := range s.Sources {
			if sourceMap[strings.ToLower(src.Name)] {
				hasSource = true
				break
			}
		}
		if !hasSource {
			return false
		}
	}

	// Query filter (in title, summary, tags)
	if query != "" {
		match := strings.Contains(strings.ToLower(s.Title), query) ||
			strings.Contains(strings.ToLower(s.Summary), query)
		if !match {
			for _, t := range s.Tags {
				if strings.Contains(strings.ToLower(t), query) {
					match = true
					break
				}
			}
		}
		if !match {
			return false
		}
	}

	return true
}

// ParseBefore parses relative age strings (e.g. "90d", "3m", "1y", "72h") or
// absolute dates (e.g. "2024-01-15", RFC3339) relative to now.
func ParseBefore(val string, now time.Time) (time.Time, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return time.Time{}, nil
	}

	lower := strings.ToLower(val)

	// Days shorthand (e.g. "90d", "30days")
	if strings.HasSuffix(lower, "d") || strings.HasSuffix(lower, "days") || strings.HasSuffix(lower, "day") {
		numStr := strings.TrimRight(lower, "days")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid day duration %q: %w", val, err)
		}
		return now.AddDate(0, 0, -num), nil
	}

	// Months shorthand (e.g. "3m", "6months", "1month")
	if strings.HasSuffix(lower, "m") || strings.HasSuffix(lower, "months") || strings.HasSuffix(lower, "month") {
		numStr := strings.TrimRight(lower, "months")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid month duration %q: %w", val, err)
		}
		return now.AddDate(0, -num, 0), nil
	}

	// Years shorthand (e.g. "1y", "2years", "1year")
	if strings.HasSuffix(lower, "y") || strings.HasSuffix(lower, "years") || strings.HasSuffix(lower, "year") {
		numStr := strings.TrimRight(lower, "years")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid year duration %q: %w", val, err)
		}
		return now.AddDate(-num, 0, 0), nil
	}

	// Weeks shorthand (e.g. "2w", "4weeks")
	if strings.HasSuffix(lower, "w") || strings.HasSuffix(lower, "weeks") || strings.HasSuffix(lower, "week") {
		numStr := strings.TrimRight(lower, "weeks")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid week duration %q: %w", val, err)
		}
		return now.AddDate(0, 0, -num*7), nil
	}

	// Standard Go time.Duration (e.g. "72h", "168h")
	if d, err := time.ParseDuration(val); err == nil {
		if d < 0 {
			return time.Time{}, fmt.Errorf("duration %q must be positive", val)
		}
		return now.Add(-d), nil
	}

	// Date format YYYY-MM-DD
	if t, err := time.Parse(time.DateOnly, val); err == nil {
		return t.UTC(), nil
	}

	// RFC3339 format
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t.UTC(), nil
	}

	return time.Time{}, fmt.Errorf("unrecognized date or age format: %q (supported: '90d', '3m', '1y', '72h', 'YYYY-MM-DD', RFC3339)", val)
}
