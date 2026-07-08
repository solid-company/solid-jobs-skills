package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/api"
	"github.com/solid-company/solid-jobs-skills/internal/jobs"
	"github.com/solid-company/solid-jobs-skills/internal/store"
)

func newOfferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "offer",
		Short: "Inspect a single cached offer by key",
		Long: "Read one offer in full from the local cache — including the job " +
			"description as plain text. Use this to go deep on a specific offer " +
			"instead of re-running a search (search output is a lean list).",
	}
	cmd.AddCommand(newOfferShowCmd())
	return cmd
}

func newOfferShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <offerKey>",
		Short: "Show one cached offer in full (description as plain text)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				o, err := s.Store.GetOffer(cmd.Context(), args[0])
				if err != nil {
					if errors.Is(err, store.ErrNotFound) {
						return fmt.Errorf("no cached offer %s (run search first)", args[0])
					}
					return fail("offer show", err)
				}
				// Serve the description as plain text; the raw HTML stays in the DB.
				o.Description = htmlToText(o.Description)
				if gf.jsonOut {
					return printJSON(o)
				}
				printOfferDetail(o)
				return nil
			})
		},
	}
}

// printOfferDetail renders a single offer as a readable text block.
func printOfferDetail(o *api.Offer) {
	fmt.Printf("%s — %s\n", o.Title, o.Company)
	fmt.Printf("Key:        %s\n", o.JobOfferKey)
	fmt.Printf("Salary:     %s\n", salaryStr(o.Salary))
	fmt.Printf("Mode:       %s\n", workMode(o))
	if len(o.Locations) > 0 {
		fmt.Printf("Location:   %s\n", strings.Join(o.Locations, ", "))
	}
	if o.ExperienceLevel != "" {
		fmt.Printf("Experience: %s\n", o.ExperienceLevel)
	}
	if len(o.Skills) > 0 {
		fmt.Println("Skills:")
		for _, sk := range o.Skills {
			fmt.Printf("  - %s (%s)\n", sk.Name, sk.Level)
		}
	}
	if len(o.Languages) > 0 {
		fmt.Println("Languages:")
		for _, l := range o.Languages {
			fmt.Printf("  - %s (%s)\n", l.Name, l.Level)
		}
	}
	if o.URL != "" {
		fmt.Printf("URL:        %s\n", o.URL)
	}
	if o.Description != "" {
		fmt.Printf("\n%s\n", o.Description)
	}
}
