package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/jobs"
	"github.com/solid-company/solid-jobs-skills/internal/store"
)

func newEvaluateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evaluate",
		Short: "Persist and read AI evaluations of offers",
		Long: "The jobs-evaluate skill scores an offer against a profile and calls " +
			"`evaluate save` to record the grade, per-dimension scores and rationale.",
	}
	cmd.AddCommand(newEvaluateSaveCmd(), newEvaluateShowCmd())
	return cmd
}

func newEvaluateSaveCmd() *cobra.Command {
	var (
		grade      string
		rationale  string
		dimensions string
	)
	cmd := &cobra.Command{
		Use:   "save <offerKey>",
		Short: "Save an evaluation for an offer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dims := map[string]any{}
			if dimensions != "" {
				if err := json.Unmarshal([]byte(dimensions), &dims); err != nil {
					return fmt.Errorf("--dimensions must be a JSON object: %w", err)
				}
			}
			if grade == "" {
				return errors.New("--grade is required")
			}
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				e, err := s.Store.SaveEvaluation(store.Evaluation{
					OfferKey:   args[0],
					ProfileID:  pid,
					Grade:      grade,
					Dimensions: dims,
					Rationale:  rationale,
				})
				if err != nil {
					return fail("evaluate save", err)
				}
				if gf.jsonOut {
					return printJSON(e)
				}
				fmt.Printf("saved evaluation %s for %s (grade %s)\n", "#"+itoa(e.ID), args[0], e.Grade)
				return nil
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&grade, "grade", "", "overall grade (A-F)")
	f.StringVar(&rationale, "rationale", "", "free-text rationale")
	f.StringVar(&dimensions, "dimensions", "", "JSON object of dimension -> score")
	return cmd
}

func newEvaluateShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <offerKey>",
		Short: "Show the latest evaluation for an offer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				e, err := s.Store.LatestEvaluation(args[0], pid)
				if err != nil {
					if errors.Is(err, store.ErrNotFound) {
						return fmt.Errorf("no evaluation for %s", args[0])
					}
					return fail("evaluate show", err)
				}
				if gf.jsonOut {
					return printJSON(e)
				}
				fmt.Printf("Offer:     %s\nGrade:     %s\nRationale: %s\n", e.OfferKey, e.Grade, e.Rationale)
				if len(e.Dimensions) > 0 {
					fmt.Println("Dimensions:")
					for k, v := range e.Dimensions {
						fmt.Printf("  %-16s %v\n", k, v)
					}
				}
				return nil
			})
		},
	}
}

func itoa(n int64) string { return fmt.Sprintf("%d", n) }
