package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/pipeline"
)

// version is overridden at build time:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always)"
var version = "dev"

// app is the resolved run context shared by every subcommand. It is built once
// in PersistentPreRunE and passed explicitly — there is no global config.
type app struct {
	cfg   *config.Config
	log   *slog.Logger
	since time.Duration // 0 means "use the configured window"
}

// pipeline builds the stage runner for this invocation.
func (a *app) pipeline() *pipeline.Pipeline { return pipeline.New(a.cfg, a.log) }

func newRootCmd() *cobra.Command {
	var (
		a         app
		configDir string
		dataDir   string
		verbose   bool
		since     string
	)

	cmd := &cobra.Command{
		Use:   "airfoil",
		Short: "High-velocity, low-drag AI intelligence pipeline",
		Long: "Airfoil ingests AI news from primary sources, clusters redundant\n" +
			"coverage into single stories, ranks them for builder relevance, and\n" +
			"writes markdown the static site renders.",
		SilenceUsage:  true,
		SilenceErrors: true,

		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// `version` must work before any config exists.
			if cmd.Name() == "version" {
				return nil
			}

			cfg, err := config.Load(config.Options{
				ConfigDir: configDir,
				DataDir:   dataDir,
				Verbose:   verbose,
			})
			if err != nil {
				return err
			}
			a.cfg = cfg
			a.log = newLogger(cfg.LogLevel, verbose)

			if since != "" {
				d, err := time.ParseDuration(since)
				if err != nil {
					return fmt.Errorf("--since %q: %w", since, err)
				}
				if d <= 0 {
					return fmt.Errorf("--since %q: must be positive", since)
				}
				a.since = d
			}
			return nil
		},
	}

	pf := cmd.PersistentFlags()
	pf.StringVar(&configDir, "config", "", "config directory (default ./config)")
	pf.StringVar(&dataDir, "data", "", "data directory (default ./data)")
	pf.BoolVarP(&verbose, "verbose", "v", false, "debug logging")
	pf.StringVar(&since, "since", "", "only consider items newer than this duration, e.g. 48h")

	cmd.AddCommand(
		newVersionCmd(),
		newDoctorCmd(&a),
		newIngestCmd(&a),
		newClusterCmd(&a),
		newRankCmd(&a),
		newWriteCmd(&a),
		newPublishCmd(&a),
		newDigestCmd(&a),
		newPurgeCmd(&a),
		newRunCmd(&a),
		newTUICmd(&a, &verbose),
	)

	return cmd
}

// newLogger returns a structured text logger. --verbose forces debug level.
func newLogger(level string, verbose bool) *slog.Logger {
	lvl := slog.LevelInfo
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}
	if verbose {
		lvl = slog.LevelDebug
	}

	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			// Wall-clock seconds are enough; the default nanosecond stamp is noise.
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.TimeOnly))
			}
			return a
		},
	}))
}
