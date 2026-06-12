package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/jobs"
	"github.com/solid-company/solid-jobs-skills/internal/store"
)

func newTrackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "track",
		Short: "Manage the application pipeline",
		Long: "Track offers through the pipeline: saved → applied → interview → " +
			"offer | rejected. Offers past their validTo are auto-expired on list.",
	}
	cmd.AddCommand(
		newTrackAddCmd(),
		newTrackListCmd(),
		newTrackSetCmd(),
		newTrackNoteCmd(),
		newTrackRmCmd(),
	)
	return cmd
}

func newTrackAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <offerKey>",
		Short: "Start tracking a cached offer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				if err := s.Store.AddTracked(args[0], pid); err != nil {
					return fail("track add", err)
				}
				fmt.Printf("tracking %s\n", args[0])
				return nil
			})
		},
	}
}

func newTrackListCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked offers",
		RunE: func(cmd *cobra.Command, args []string) error {
			if status != "" && !store.ValidStatus(status) {
				return fmt.Errorf("invalid status %q", status)
			}
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				items, err := s.Store.ListTracked(pid, status)
				if err != nil {
					return fail("track list", err)
				}
				if gf.jsonOut {
					return printJSON(items)
				}
				if len(items) == 0 {
					fmt.Println("no tracked offers")
					return nil
				}
				w := newTabWriter()
				fmt.Fprintln(w, "KEY\tSTATUS\tGRADE\tTITLE\tCOMPANY")
				for _, t := range items {
					grade := t.Grade
					if grade == "" {
						grade = "-"
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						t.OfferKey, t.Status, grade, truncate(t.Title, 40), truncate(t.Company, 24))
				}
				w.Flush()
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	return cmd
}

func newTrackSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <offerKey> <status>",
		Short: "Set the status of a tracked offer",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !store.ValidStatus(args[1]) {
				return fmt.Errorf("invalid status %q (saved|applied|interview|offer|rejected|expired)", args[1])
			}
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				if err := s.Store.SetStatus(args[0], pid, args[1]); err != nil {
					if errors.Is(err, store.ErrNotFound) {
						return fmt.Errorf("offer %s is not tracked", args[0])
					}
					return fail("track set", err)
				}
				fmt.Printf("%s → %s\n", args[0], args[1])
				return nil
			})
		},
	}
}

func newTrackNoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "note <offerKey> <text>",
		Short: "Attach a note to a tracked offer",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			note := joinArgs(args[1:])
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				if err := s.Store.SetNotes(args[0], pid, note); err != nil {
					return fail("track note", err)
				}
				fmt.Printf("note saved for %s\n", args[0])
				return nil
			})
		},
	}
}

func newTrackRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <offerKey>",
		Short: "Stop tracking an offer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				if err := s.Store.RemoveTracked(args[0], pid); err != nil {
					return fail("track rm", err)
				}
				fmt.Printf("untracked %s\n", args[0])
				return nil
			})
		},
	}
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
