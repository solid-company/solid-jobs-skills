package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/jobs"
	"github.com/solid-company/solid-jobs-skills/internal/store"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage user profiles",
		Long: "Profiles hold your CV/preferences text used to score offers. The " +
			"`default` profile is created automatically; add more for distinct job targets.",
	}
	cmd.AddCommand(
		newProfileAddCmd(),
		newProfileListCmd(),
		newProfileShowCmd(),
		newProfileImportCmd(),
		newProfileSetDefaultCmd(),
	)
	return cmd
}

func newProfileAddCmd() *cobra.Command {
	var (
		file     string
		makeDflt bool
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create a profile, optionally from a markdown file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := ""
			if file != "" {
				b, err := os.ReadFile(file)
				if err != nil {
					return fail("read file", err)
				}
				content = string(b)
			}
			return withService(func(s *jobs.Service) error {
				p, err := s.Store.AddProfile(cmd.Context(), args[0], content, makeDflt)
				if err != nil {
					return fail("profile add", err)
				}
				fmt.Printf("created profile %q (id %d, default=%v)\n", p.Name, p.ID, p.IsDefault)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "markdown file to load as content")
	cmd.Flags().BoolVar(&makeDflt, "default", false, "make this the default profile")
	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				ps, err := s.Store.ListProfiles(cmd.Context())
				if err != nil {
					return fail("profile list", err)
				}
				if gf.jsonOut {
					return printJSON(ps)
				}
				w := newTabWriter()
				fmt.Fprintln(w, "ID\tNAME\tDEFAULT\tCHARS")
				for _, p := range ps {
					def := ""
					if p.IsDefault {
						def = "*"
					}
					fmt.Fprintf(w, "%d\t%s\t%s\t%d\n", p.ID, p.Name, def, len(p.Content))
				}
				w.Flush()
				return nil
			})
		},
	}
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show a profile (default profile if name omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			} else {
				name = gf.profile
			}
			return withService(func(s *jobs.Service) error {
				var p *store.Profile
				var err error
				if name == "" {
					p, err = s.Store.DefaultProfile(cmd.Context())
				} else {
					p, err = s.Store.GetProfileByName(cmd.Context(), name)
				}
				if err != nil {
					return fail("profile show", err)
				}
				if gf.jsonOut {
					return printJSON(p)
				}
				fmt.Printf("# %s%s\n\n%s\n", p.Name, defaultMark(p.IsDefault), p.Content)
				return nil
			})
		},
	}
}

func newProfileImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <name> <file>",
		Short: "Replace a profile's content from a markdown file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := os.ReadFile(args[1])
			if err != nil {
				return fail("read file", err)
			}
			return withService(func(s *jobs.Service) error {
				p, err := s.Store.GetProfileByName(cmd.Context(), args[0])
				if err != nil {
					return fail("profile import", err)
				}
				if err := s.Store.SetProfileContent(cmd.Context(), p.ID, string(b)); err != nil {
					return fail("profile import", err)
				}
				fmt.Printf("imported %d chars into %q\n", len(b), args[0])
				return nil
			})
		},
	}
}

func newProfileSetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default <name>",
		Short: "Make a profile the default",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				p, err := s.Store.GetProfileByName(cmd.Context(), args[0])
				if err != nil {
					return fail("set-default", err)
				}
				if err := s.Store.SetDefaultProfile(cmd.Context(), p.ID); err != nil {
					return fail("set-default", err)
				}
				fmt.Printf("default profile is now %q\n", args[0])
				return nil
			})
		},
	}
}

func defaultMark(b bool) string {
	if b {
		return " (default)"
	}
	return ""
}
