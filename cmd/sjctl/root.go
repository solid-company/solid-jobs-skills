package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/jobs"
	"github.com/solid-company/solid-jobs-skills/internal/store"
)

// global flags shared by all subcommands.
type globalFlags struct {
	dbPath   string
	campaign string
	jsonOut  bool
	profile  string
}

var gf globalFlags

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sjctl",
		Short:         "SOLID.Jobs search, tracking and evaluation engine",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Keep an installed sjctl current without a manual reinstall: at most once
		// a day, check for a newer release and, if one exists, download and install
		// it. This runs synchronously before the command — the once-a-day gate
		// makes the common case a single cheap API call, but the run that actually
		// finds an update blocks while the new binary downloads (the update itself
		// only takes effect on the next invocation). The version and update
		// commands opt out so they never trigger a check.
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			if autoUpdateExempt(cmd.Name()) {
				return
			}
			maybeAutoUpdate()
		},
	}
	pf := root.PersistentFlags()
	pf.StringVar(&gf.dbPath, "db", defaultDBPath(), "path to SQLite database")
	pf.StringVar(&gf.campaign, "campaign", "solid-jobs-skills", "API campaign parameter")
	pf.BoolVar(&gf.jsonOut, "json", false, "output JSON instead of a table")
	pf.StringVar(&gf.profile, "profile", "", "profile name (default: the default profile)")

	root.AddCommand(
		newSearchCmd(),
		newTrackCmd(),
		newSyncCmd(),
		newStatsCmd(),
		newMarketCmd(),
		newProfileCmd(),
		newWatchCmd(),
		newEvaluateCmd(),
		newVersionCmd(),
		newUpdateCmd(),
	)
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the sjctl version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), version)
			return nil
		},
	}
}

// defaultDBPath resolves where the SQLite database lives. SJCTL_DB wins so
// callers can pin a location; otherwise the db lives under the user's home so
// the binary works from any working directory (it is installed outside the repo
// when distributed via release archives). The relative path is only a last
// resort if the home directory cannot be determined.
func defaultDBPath() string {
	if v := os.Getenv("SJCTL_DB"); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".solid-jobs-skills", "solidjobs.db")
	}
	return "data/solidjobs.db"
}

// withService opens the store and runs fn with a ready Service, closing the
// store afterwards.
func withService(fn func(*jobs.Service) error) error {
	st, err := store.Open(gf.dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	return fn(jobs.New(st, gf.campaign))
}

// printJSON writes v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// fail wraps an error with context for consistent messaging.
func fail(what string, err error) error {
	return fmt.Errorf("%s: %w", what, err)
}
