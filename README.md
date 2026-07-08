# solid-jobs-skills

AI job-search command center for the [SOLID.Jobs](https://solid.jobs) public API.
A Go CLI (`sjctl`) fetches, caches and tracks offers; Claude Code skills turn
natural language into searches, score offers against your profile, and surface
new matches.

Powered by the [SOLID.Jobs API client](https://github.com/solid-company/solid-jobs-client).

## Install

**Via [skills.sh](https://skills.sh)** (recommended) — installs the four skills
into Claude Code; the first skill run downloads a checksum-verified `sjctl`
binary into `~/.solid-jobs-skills/bin`:

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

## Quick start (from source)

```sh
go build -o sjctl ./cmd/sjctl
./sjctl search -d IT --term golang --remote --min-salary 20000

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
```

In Claude Code, use the skills: `/jobs-search`, `/jobs-evaluate`, `/jobs-track`,
`/jobs-digest`.

## Features (MVP)

- **Search** offers by division, skill, city, experience, salary, remote.
- **Track** applications through saved → applied → interview → offer/rejected,
  with auto-expiry past `validTo`.
- **Evaluate** offers A–F against your profile (via the `jobs-evaluate` skill),
  persisted for later review.
- **Watch & digest** saved searches, reporting only offers you haven't seen.
- **Stats**: salary min/median/max, remote share, counts by experience level.
- **Market**: live server-side statistics for a division, category, specialization
  or city — demand, salary bands, experience mix and top locations/skills.

See [CLAUDE.md](CLAUDE.md) for architecture and the full command reference.

## License

MIT
