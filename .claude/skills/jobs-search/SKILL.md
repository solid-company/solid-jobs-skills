---
name: jobs-search
description: Search SOLID.Jobs offers from a natural-language request. Translates phrases like "senior Go, remote, 25k+ in Warsaw" into sjctl search flags, runs the query, and presents the best matches. Use when the user wants to find or browse job offers.
---

# jobs-search

Turn a natural-language job request into a `sjctl search` invocation, run it, and present results.

## Running sjctl

Resolve the `sjctl` binary in this order and use the first that works:

1. `sjctl` on `PATH`
2. `~/.solid-jobs-skills/bin/sjctl` (`sjctl.exe` on Windows) — where the installer puts it
3. `./sjctl` / `./sjctl.exe` in the current repo (local dev)
4. If none exist, install it, then use the path the installer prints on stdout:
   - macOS/Linux: `curl -fsSL https://raw.githubusercontent.com/solid-company/solid-jobs-skills/main/scripts/install-sjctl.sh | bash`
   - Windows: `irm https://raw.githubusercontent.com/solid-company/solid-jobs-skills/main/scripts/install-sjctl.ps1 | iex`
   - Dev fallback (Go installed, inside the repo): `go run ./cmd/sjctl`

The installer downloads a checksum-verified release binary into `~/.solid-jobs-skills/bin`. The database lives at `~/.solid-jobs-skills/solidjobs.db` regardless of working directory.

Always pass `--json` so you can parse and reason over the results, then summarize for the user.

`search --json` returns a **lean list** — key, title, company, salary, mode, location, experience, skills, languages — with the full HTML job description omitted to save tokens. The complete offer is still cached; when you need the full description for one offer (e.g. before `/jobs-evaluate` or `/jobs-interview`), read it on demand with `sjctl offer show <jobOfferKey> --json`, which serves the description as plain text from the cache with no extra API call.

## Mapping language to flags

| User says | Flag |
|-----------|------|
| division (IT, Engineering, Marketing, Sales, HR, Logistics, Finances, Other) | `-d <Division>` (default IT) |
| a role/keyword ("Go", "React", "DevOps") | `--term <kw>` (repeatable) |
| a category ("Developer", "Tester") | `--category <Cat>` |
| a tech subcategory ("Java", "DotNet") | `--subcategory <Sub>` |
| seniority ("senior", "junior", "regular") | `--experience <Level>` |
| a city ("Warsaw", "Poznań") | `--city <City>` (repeatable) |
| "remote" | `--remote` |
| "at least 20k", "25000+" | `--min-salary 20000` |
| "show more" / page N | `--page-size`, `--page-index` |
| "highest paid first" | `--sort salaryFrom --sort-dir desc` |

Divisions and experience levels are case-sensitive (e.g. `Senior`, not `senior`).

## Flow

1. Parse the request into flags. If the division is ambiguous, default to IT and say so.
2. Run e.g. `sjctl search -d IT --term golang --remote --min-salary 20000 --page-size 30 --json`.
3. Summarize the top matches: title, company, salary, work mode, location, and the `jobOfferKey` (needed for tracking/evaluating).
4. Offer next steps: track an offer (`/jobs-track`) or evaluate fit (`/jobs-evaluate`).

Searching caches offers locally, so the keys you show can be tracked or evaluated immediately without re-querying.

## Notes

- The API rate limit is 300 req/min; don't loop searches needlessly.
- If the API returns nothing, loosen filters (drop `--min-salary` or a `--term`) and retry once.
