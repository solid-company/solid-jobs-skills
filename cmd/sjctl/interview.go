package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/jobs"
	"github.com/solid-company/solid-jobs-skills/internal/store"
)

// strictUnmarshal decodes JSON into v and rejects unknown keys, so a typo in a
// hand-authored --gaps/--ask/--questions payload fails loudly instead of being
// silently dropped.
func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func newInterviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "interview",
		Short: "Persist and drive interview preparation for an offer",
		Long: "The jobs-interview skill builds a prep pack (gap analysis, question bank, " +
			"questions to ask) against a profile and calls `interview save` to record it. " +
			"`practice` and `rate` drive the mock-interview loop; readiness is recomputed " +
			"from per-question confidence.",
	}
	cmd.AddCommand(
		newInterviewSaveCmd(),
		newInterviewShowCmd(),
		newInterviewListCmd(),
		newInterviewPracticeCmd(),
		newInterviewRateCmd(),
	)
	return cmd
}

func newInterviewSaveCmd() *cobra.Command {
	var (
		readiness int
		gaps      string
		ask       string
		summary   string
		questions string
	)
	cmd := &cobra.Command{
		Use:   "save <offerKey>",
		Short: "Save an interview prep session for an offer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var parsedGaps []store.Gap
			if gaps != "" {
				if err := strictUnmarshal([]byte(gaps), &parsedGaps); err != nil {
					return fmt.Errorf("--gaps must be a JSON array of {skill,severity,note}: %w", err)
				}
			}
			var parsedAsk []string
			if ask != "" {
				if err := strictUnmarshal([]byte(ask), &parsedAsk); err != nil {
					return fmt.Errorf("--ask must be a JSON array of strings: %w", err)
				}
			}
			var parsedQuestions []store.InterviewQuestionInput
			if questions != "" {
				if err := strictUnmarshal([]byte(questions), &parsedQuestions); err != nil {
					return fmt.Errorf("--questions must be a JSON array of {category,question,difficulty,talkingPoints}: %w", err)
				}
			}
			if !store.ValidReadiness(readiness) {
				return fmt.Errorf("--readiness %d out of range (want 0-100)", readiness)
			}
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(cmd.Context(), gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				p, err := s.Store.SaveInterviewPrep(cmd.Context(), store.InterviewPrepInput{
					OfferKey:       args[0],
					ProfileID:      pid,
					Readiness:      readiness,
					Gaps:           parsedGaps,
					QuestionsToAsk: parsedAsk,
					Summary:        summary,
					Questions:      parsedQuestions,
				})
				if err != nil {
					return fail("interview save", err)
				}
				if gf.jsonOut {
					return printJSON(p)
				}
				fmt.Printf("saved interview prep #%d for %s (readiness %d, %d questions)\n",
					p.ID, p.OfferKey, p.Readiness, len(p.Questions))
				return nil
			})
		},
	}
	f := cmd.Flags()
	f.IntVar(&readiness, "readiness", 0, "initial readiness score (0-100)")
	f.StringVar(&gaps, "gaps", "", "JSON array of gaps: [{skill,severity,note}]")
	f.StringVar(&ask, "ask", "", "JSON array of questions to ask the recruiter")
	f.StringVar(&summary, "summary", "", "gap-analysis narrative / strengths")
	f.StringVar(&questions, "questions", "", "JSON array of {category,question,difficulty,talkingPoints}")
	return cmd
}

func newInterviewShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <offerKey>",
		Short: "Show the latest interview prep for an offer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(cmd.Context(), gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				p, err := s.Store.LatestInterviewPrep(cmd.Context(), args[0], pid)
				if err != nil {
					if errors.Is(err, store.ErrNotFound) {
						return fmt.Errorf("no interview prep for %s (run /jobs-interview first)", args[0])
					}
					return fail("interview show", err)
				}
				qs, err := s.Store.QuestionsForPrep(cmd.Context(), p.ID)
				if err != nil {
					return fail("interview show", err)
				}
				p.Questions = qs
				if gf.jsonOut {
					return printJSON(p)
				}
				fmt.Printf("Offer:     %s\nReadiness: %d/100\n", p.OfferKey, p.Readiness)
				if p.Summary != "" {
					fmt.Printf("Summary:   %s\n", p.Summary)
				}
				if len(p.Gaps) > 0 {
					fmt.Println("Gaps:")
					for _, g := range p.Gaps {
						fmt.Printf("  [%s] %s — %s\n", g.Severity, g.Skill, g.Note)
					}
				}
				if len(p.QuestionsToAsk) > 0 {
					fmt.Println("Ask the recruiter:")
					for _, a := range p.QuestionsToAsk {
						fmt.Printf("  - %s\n", a)
					}
				}
				if len(qs) > 0 {
					fmt.Println("Questions:")
					for _, q := range qs {
						fmt.Printf("  #%d [%s/%s] conf %d/5  %s\n",
							q.ID, q.Category, q.Difficulty, q.Confidence, q.Question)
					}
				}
				return nil
			})
		},
	}
}

func newInterviewListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List interview preps for the profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(cmd.Context(), gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				preps, err := s.Store.ListInterviewPreps(cmd.Context(), pid)
				if err != nil {
					return fail("interview list", err)
				}
				if gf.jsonOut {
					return printJSON(preps)
				}
				if len(preps) == 0 {
					fmt.Println("no interview preps yet")
					return nil
				}
				for _, p := range preps {
					fmt.Printf("%-24s  %3d/100  %s @ %s\n", p.OfferKey, p.Readiness, p.Title, p.Company)
				}
				return nil
			})
		},
	}
}

func newInterviewPracticeCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "practice <offerKey>",
		Short: "Show the next questions to drill (lowest confidence first)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				pid, err := s.ResolveProfileID(cmd.Context(), gf.profile)
				if err != nil {
					return fail("resolve profile", err)
				}
				qs, err := s.Store.NextPracticeQuestions(cmd.Context(), args[0], pid, limit)
				if err != nil {
					if errors.Is(err, store.ErrNotFound) {
						return fmt.Errorf("no interview prep for %s (run /jobs-interview first)", args[0])
					}
					return fail("interview practice", err)
				}
				if gf.jsonOut {
					return printJSON(qs)
				}
				if len(qs) == 0 {
					fmt.Println("no questions to practice")
					return nil
				}
				for _, q := range qs {
					fmt.Printf("#%d [%s/%s] conf %d/5\n  %s\n", q.ID, q.Category, q.Difficulty, q.Confidence, q.Question)
					if q.TalkingPoints != "" {
						fmt.Printf("  hints: %s\n", q.TalkingPoints)
					}
				}
				return nil
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "how many questions to return")
	return cmd
}

func newInterviewRateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rate <questionId> <0-5>",
		Short: "Record confidence for a question and recompute readiness",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			qid, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("questionId must be an integer: %w", err)
			}
			conf, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("confidence must be an integer 0-5: %w", err)
			}
			return withService(func(s *jobs.Service) error {
				p, err := s.Store.RateQuestion(cmd.Context(), qid, conf)
				if err != nil {
					if errors.Is(err, store.ErrNotFound) {
						return fmt.Errorf("no question #%d", qid)
					}
					return fail("interview rate", err)
				}
				if gf.jsonOut {
					return printJSON(p)
				}
				fmt.Printf("rated question #%d at %d/5 — readiness now %d/100\n", qid, conf, p.Readiness)
				return nil
			})
		},
	}
}
