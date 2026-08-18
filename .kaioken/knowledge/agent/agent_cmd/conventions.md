All logic lives in internal/; main.go is intentionally thin
Configuration loaded once in PersistentPreRunE, stored in app struct, passed explicitly to subcommands — no global config
Persistent flags: --config, --data, --verbose/-v, --since (duration string parsed in PreRunE)
SilenceUsage and SilenceErrors true on root command for clean output
Logger uses slog with TextHandler; timestamps formatted as HH:MM:SS via ReplaceAttr
Version injected at build time via -ldflags "-X main.version=..."
Doctor command validates config only (Phase 0); live provider pings deferred to network layer
Blocking errors collected in slice and returned as single error joined by semicolons
Advisory checks print warnings but do not fail the command
Sources grouped by tier in doctor output for quick scanning
