Config is constructed once at startup in root command and passed explicitly — no globals, no os.Getenv outside config package.
Precedence order is hardcoded in Load: CLI Options > environment variable > default literal.
Environment variables use AIRFOIL_ prefix for app-specific knobs (AIRFOIL_DATA_DIR, AIRFOIL_CONFIG_DIR, AIRFOIL_LOG_LEVEL, AIRFOIL_EMBEDDER, AIRFOIL_STORIES_DIR); provider keys use standard names (GEMINI_API_KEY, OLLAMA_HOST, etc.).
loadDotEnv silently ignores missing .env; existing env vars are never overwritten (CI wins).
JSON config files live in ConfigDir and are mandatory — parse or validation failure is a hard startup error.
Source IDs must be unique, non-empty; Type must be one of five constants; Tier 1-5; Enabled bool gates inclusion.
SourceOptions is a union struct; each ingest adapter reads only its fields (Queries/MinPoints/HoursBack for HN, Limit/MinScore for Reddit, etc.).
Scoring struct mirrors scoring.json exactly; all tuning knobs live here to avoid rebuilds. Scoring.problems() validates ranges and presence.
Keywords struct mirrors keywords.json; BuilderSignals, BuilderStructuralSignals, HypeSignals are case-insensitive matched against title+excerpt; TopicTags maps topic -> signal list.
Derived paths use filepath.Join on DataDir; StoriesDir defaults to site/src/content/stories but can be overridden via AIRFOIL_STORIES_DIR.
Validation aggregates all problems and returns a single formatted error with bullet list.
