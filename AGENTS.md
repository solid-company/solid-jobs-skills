# Agent guide — solid-jobs-skills

These job-search skills are plain markdown that drive a single CLI (`sjctl`). They
work with **any** coding agent, not just Claude Code — if you're Codex or another
agent that reads `AGENTS.md`, this file is your entry point.

## How to use the skills

Each skill is a markdown playbook under `.claude/skills/<name>/SKILL.md`. When the
user's request matches one, **open that file and follow it** — the frontmatter
`description` tells you when each applies.

| Skill | Use when the user wants to… |
|-------|------------------------------|
| `jobs-search` | find or browse job offers |
| `jobs-create-profile` | set up / update / switch their candidate profile |
| `jobs-evaluate` | judge whether a specific offer is worth it |
| `jobs-track` | track applications through a pipeline |
| `jobs-interview` | prepare for an interview / run a mock interview |
| `jobs-digest` | see what's new since last check |
| `jobs-market` | ask a standalone pay/demand/trend question — not tied to browsing or evaluating a specific offer |

Routing files: `jobs-search/SKILL.md`, `jobs-create-profile/SKILL.md`,
`jobs-evaluate/SKILL.md`, `jobs-track/SKILL.md`, `jobs-interview/SKILL.md`,
`jobs-digest/SKILL.md`, `jobs-market/SKILL.md`.

## Running sjctl

Every skill shells out to `sjctl`. Resolve the binary in this order and use the
first that works:

1. `sjctl` on `PATH`
2. `~/.solid-jobs-skills/bin/sjctl` (`sjctl.exe` on Windows) — installer location
3. `./sjctl` / `./sjctl.exe` in the repo (local dev)
4. If none exist, install it, then use the path the installer prints:
   - macOS/Linux: `curl -fsSL https://raw.githubusercontent.com/solid-company/solid-jobs-skills/main/scripts/install-sjctl.sh | bash`
   - Windows: `irm https://raw.githubusercontent.com/solid-company/solid-jobs-skills/main/scripts/install-sjctl.ps1 | iex`
   - Dev fallback (Go installed, inside the repo): `go run ./cmd/sjctl`

Pass `--json` to any command when you need to parse and reason over results. The
database lives at `~/.solid-jobs-skills/solidjobs.db` regardless of working
directory (override with `SJCTL_DB`).

## Notes

- The skills assume only a shell and `sjctl`; they use no Claude-specific tools, so
  the instructions transfer verbatim to other agents.
- API rate limit is 300 req/min — don't loop requests needlessly.
- See `CLAUDE.md` for architecture and the full `sjctl` command map.
