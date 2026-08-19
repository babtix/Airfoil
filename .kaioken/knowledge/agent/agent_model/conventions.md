Item.ID is sha256(canonicalURL)[:16]; Excerpt capped at 300 chars by normalize package (R1)
All struct fields use explicit json tags; Centroid in Cluster uses json:"-" to exclude from serialization
Story tiers are the string constants TierMajor, TierNotable, TierMinor — no other values
SourceType(tier int) is the single authority for tier→category mapping (lab/press/community)
State.SeenURLs values are UTC dates formatted with time.DateOnly (YYYY-MM-DD)
NewState() returns a non-nil State with initialized maps; callers must not assume nil maps
Metrics fields use omitempty; StorySource.Metrics is a pointer for optional presence
IndexEntry is a strict subset of Story fields for feed/top/ship consumption
