---
name: jobs-digest
description: Run saved searches (watches), find offers not seen before, and summarize the best new matches against the user's profile. Use when the user asks "what's new?", "any new jobs?", or wants a periodic digest of fresh offers.
---

# jobs-digest

Surface and triage new offers since the last check.

Resolve `sjctl` in this order: (1) on `PATH`, (2) `~/.solid-jobs-skills/bin/sjctl[.exe]`, (3) `./sjctl[.exe]` in the repo. If none exist, install it and use the printed path:
- macOS/Linux: `curl -fsSL https://raw.githubusercontent.com/solid-company/solid-jobs-skills/main/scripts/install-sjctl.sh | bash`
- Windows: `irm https://raw.githubusercontent.com/solid-company/solid-jobs-skills/main/scripts/install-sjctl.ps1 | iex`
- Dev fallback (Go, inside repo): `go run ./cmd/sjctl`

## Prerequisite: watches

Digest is driven by saved searches. If none exist (`sjctl watch list --json` is empty), help the user create one from their typical criteria:

```
sjctl watch add my-go-roles -d IT --term golang --remote --min-salary 20000
```

## Flow

1. `sjctl sync --json` — runs every watch, caches results, returns only offers not previously reported (the `new` array). A second run returns nothing until genuinely new offers appear.
2. If `new` is empty, tell the user there's nothing new and stop.
3. Otherwise, load the profile (`sjctl profile show --json`) and quickly rank the new offers by fit using the same dimensions as `/jobs-evaluate` (skill match, salary, seniority, work mode). Don't over-analyze — this is a triage pass.
4. Present a short digest: for each strong new offer, one line with title, company, salary, work mode, and why it fits, plus the `jobOfferKey`.
5. Offer to track the best ones (`/jobs-track`) or do a full evaluation (`/jobs-evaluate`).

## Notes

- `sync` marks offers as seen as a side effect, so re-running won't re-surface them — do a full pass in one go.
- Keep API usage modest; the rate limit is 300 req/min.
