# solid-jobs-skills

[![Available on skills.sh](https://img.shields.io/badge/skills.sh-solid--jobs--skills-6f42c1)](https://skills.sh/solid-company/solid-jobs-skills)
[![Downloads](https://img.shields.io/github/downloads/solid-company/solid-jobs-skills/total)](https://github.com/solid-company/solid-jobs-skills/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

AI job-search command center for the [SOLID.Jobs](https://solid.jobs) public API.
A Go CLI (`sjctl`) fetches, caches and tracks offers; a set of agent skills turn
natural language into searches, build your candidate profile, score offers against
it, and surface new matches — with **clickable links** straight to each posting.

Powered by the [SOLID.Jobs API client](https://github.com/solid-company/solid-jobs-client).

## Install

**Via [skills.sh](https://skills.sh)** (recommended) — installs the skills into
Claude Code; the first skill run downloads a checksum-verified `sjctl` binary into
`~/.solid-jobs-skills/bin`:

```sh
npx skills add solid-company/solid-jobs-skills
```

**Binary only** — pull the latest release for your platform without skills.sh:

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/solid-company/solid-jobs-skills/main/scripts/install-sjctl.sh | bash
# Windows (PowerShell)
irm https://raw.githubusercontent.com/solid-company/solid-jobs-skills/main/scripts/install-sjctl.ps1 | iex
```

Pin a version with `SJCTL_VERSION=v0.1.0`, change the install dir with
`SJCTL_BIN_DIR`. The database always lives at `~/.solid-jobs-skills/solidjobs.db`
(override with `SJCTL_DB`), so `sjctl` works from any directory.

Releases are built by [GoReleaser](https://goreleaser.com) on every `v*` tag for
linux/darwin/windows × amd64/arm64. The binary is statically linked (pure-Go
SQLite, `CGO_ENABLED=0`).

## The skills

Seven skills, each a natural-language playbook over `sjctl`. A typical loop:
**search** offers → set up your **profile** → **evaluate** the interesting ones →
**track** what you apply to → **prep** for the interview → **digest** new matches
as they appear.

### `/jobs-search` — find & browse offers

Translates a plain-language request into a `sjctl search` query, runs it, and shows
the best matches as a clickable table.

- **Triggers on:** "find me senior Go remote jobs", "show backend roles in Warsaw
  25k+", "any React jobs?"
- **Under the hood:** maps your words to flags — `-d <Division>`, `--term`,
  `--category`, `--experience`, `--city`, `--remote`, `--min-salary`, sorting,
  paging — then runs `sjctl search … --json`. Divisions and experience levels are
  case-sensitive (IT, Engineering, Marketing, Sales, HR, Logistics, Finances,
  Other).
- **Output:** a markdown table where each title links to the live posting on
  solid.jobs, plus company, salary, work mode, location, and the `jobOfferKey`
  used by the other skills. Searching caches offers locally, so those keys are
  immediately trackable/evaluable.
- **Chains to:** `/jobs-evaluate` (is it worth it?) or `/jobs-track` (save it).

### `/jobs-create-profile` — build & manage your candidate profile

Turns a self-description or pasted CV into a stored profile. The profile is the
yardstick every evaluation uses, and it lives in the database, not a file.

- **Triggers on:** "set up my profile", "here's my CV", "update my preferences",
  "switch to my contracting profile", "what's in my profile?"
- **What it captures:** role & seniority, skills (strong/working/learning),
  compensation (minimum, target, contract type), location & work mode,
  deal-breakers, nice-to-haves — the same dimensions `/jobs-evaluate` scores.
- **Under the hood:** fills the `config/profile.md` layout from what you said,
  writes it to a temp file, and saves it with `sjctl profile add <name> --file …`
  (or `sjctl profile import` to update). Manage profiles with `sjctl profile list`,
  `profile show`, and `profile set-default` to switch the active one.
- **Chains to:** everything — `/jobs-evaluate` and `/jobs-digest` grade against the
  active profile (or any named one via `--profile`).

### `/jobs-evaluate` — is this offer worth it?

Scores a cached offer against your profile, gives an A–F verdict, and saves it.

- **Triggers on:** "is this offer worth it?", "evaluate this job", "rate this one".
- **Rubric** — six weighted dimensions, each A–F:

  | Dimension | Weight | What it checks |
  |-----------|--------|----------------|
  | Skill match | 30% | required skills vs your strong/working skills |
  | Salary fit | 25% | offer band vs your minimum and target |
  | Seniority fit | 15% | offer level vs your seniority |
  | Work mode / location | 15% | remote/hybrid/onsite and city vs preference |
  | Contract type | 10% | B2B vs employment vs preference |
  | Red flags | 5% | vague scope, missing salary, deal-breakers hit |

  A violated **deal-breaker** caps the overall grade at D. Scale: A = apply now,
  B = strong, C = worth a look, D = weak, F = skip.
- **Under the hood:** reads the cached offer plus `sjctl profile show --json`, then
  persists the verdict with `sjctl evaluate save <offerKey> --grade … --dimensions
  … --rationale …`. Saved grades show up in the tracker board.
- **Chains to:** `/jobs-track` when the grade is good.

### `/jobs-track` — your application pipeline

Moves offers through `saved → applied → interview → offer / rejected`, with notes
and auto-expiry once an offer's `validTo` passes.

- **Triggers on:** "I applied to this", "move it to interview", "show my pipeline",
  "where am I with my applications?"
- **Under the hood:** `sjctl track add/list/set/note/rm`; `list` auto-expires stale
  offers first so the board always reflects what's still open.
- **Output:** a clickable board grouped by status — title (linked to the posting),
  company, status, grade (from the latest evaluation), and key.
- **Chains from:** `/jobs-search`, `/jobs-evaluate`, `/jobs-digest`.

### `/jobs-interview` — prepare for the interview

Turns a cached offer plus your profile into an interview prep pack, persists it,
and can run a mock interview that tracks readiness.

- **Triggers on:** "prepare me for this interview", "interview questions for this
  job", "mock interview", "przygotuj mnie do rozmowy".
- **What it produces:** a gap analysis with a 0–100 readiness score, a tailored
  question bank (technical per required skill + behavioral/situational with talking
  points), and questions to ask the recruiter.
- **Under the hood:** reads the cached offer, `sjctl profile show --json`, and the
  latest evaluation as a seed, then persists the pack with `sjctl interview save
  <offerKey> --readiness … --gaps … --ask … --questions …`. `sjctl interview
  practice` surfaces the lowest-confidence questions and `interview rate
  <questionId> <0-5>` recomputes readiness from per-question confidence.
- **Chains from:** `/jobs-evaluate` (reuses the grade and dimensions as a seed).

### `/jobs-digest` — what's new since last time

Runs your saved searches (watches), reports only offers you haven't seen, and
triages them against your profile.

- **Triggers on:** "what's new?", "any new jobs?", "give me today's digest".
- **Prerequisite:** at least one watch (`sjctl watch add my-go-roles -d IT --term
  golang --min-salary 20000`).
- **Under the hood:** `sjctl sync --json` runs every watch, caches results, and
  returns only the `new` array; the skill ranks those by fit (same dimensions as
  evaluate) and marks them seen so they won't resurface.
- **Output:** a short clickable digest table — title (linked), company, salary,
  mode, why it fits, key.
- **Chains to:** `/jobs-track` or `/jobs-evaluate` for the best ones.

### `/jobs-market` — pay, demand, and trends

Answers standalone market questions by picking between a live scope snapshot
and a role's yearly trend report.

- **Triggers on:** "what do React devs earn?", "is Go in demand in Warsaw?",
  "has Golang's pay changed over the years?".
- **Under the hood:** current/typical pay or demand for a division, category,
  subcategory, subcategory group, or city → `sjctl market <scopeKind>
  <scopeKey> --json`. A trend over time for a single role/skill name → `sjctl
  market raport <role> --json` (a 3-year yearly breakdown, no `--fields`).
  Both reflect the whole live market, independent of the local offer cache.
- **Output:** a salary-band/demand summary for a snapshot, or a compact
  year-over-year table for a trend.
- **Chains from:** `/jobs-search` and `/jobs-evaluate`, which pull in market
  context inline but hand off standalone market questions here.

## Use with other agents

The skills are plain markdown that shell out to `sjctl` with no Claude-specific
tooling, so they work with any coding agent. skills.sh installs them under
`.claude/skills/`; agents that read an `AGENTS.md` (e.g. Codex) get the same
playbooks via the repo-root [`AGENTS.md`](AGENTS.md), which lists the skills and the
`sjctl` resolution order. One source of truth, every agent.

## Quick start (from source)

```sh
go build -o sjctl ./cmd/sjctl

# search — table now includes a clickable LINK column
./sjctl search -d IT --term golang --remote --min-salary 20000
# KEY     TITLE                COMPANY   SALARY            MODE    LOCATION   LINK
# abc123  Senior Go Engineer   Acme      22000-28000 PLN   remote  Remote     https://solid.jobs/…

# set up your profile, then evaluate offers against it
./sjctl profile import default config/profile.md

# track an offer through the pipeline
./sjctl track add <jobOfferKey>
./sjctl track set <jobOfferKey> applied

# save a watch and check for new offers
./sjctl watch add my-go-roles -d IT --term golang --remote
./sjctl sync

# live market statistics for a scope (no prior search needed)
./sjctl market subcategory React
./sjctl market division IT --fields salary,demand --json
./sjctl market raport Golang --json   # 3-year yearly trend for a role
```

In Claude Code, use the skills: `/jobs-search`, `/jobs-create-profile`,
`/jobs-evaluate`, `/jobs-track`, `/jobs-interview`, `/jobs-digest`, `/jobs-market`.

## Features

- **Search** offers by division, skill, city, experience, salary, remote.
- **Track** applications through saved → applied → interview → offer/rejected,
  with auto-expiry past `validTo`.
- **Evaluate** offers A–F against your profile (via the `jobs-evaluate` skill),
  persisted for later review.
- **Interview prep** (via the `jobs-interview` skill): gap analysis with a
  readiness score, a tailored question bank, questions to ask the recruiter, and
  a mock-interview loop where per-question confidence recomputes readiness.
- **Watch & digest** saved searches, reporting only offers you haven't seen.
- **Stats**: salary min/median/max, remote share, counts by experience level.
- **Market**: live server-side statistics for a division, category, specialization
  or city — demand, salary bands, experience mix and top locations/skills. Plus a
  3-year yearly trend report for a single role via `market raport <role>`.

See [CLAUDE.md](CLAUDE.md) for architecture and the full command reference, and
[AGENTS.md](AGENTS.md) for using the skills with other agents.

## License

MIT
