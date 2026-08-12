---
name: jobs-market
description: Answer market-intelligence questions about pay, demand, and trends by querying the SOLID.Jobs live market-statistics API — choosing contextually between a current-snapshot scope query and a multi-year role trend report. Use when the user asks "what's the market like for X", "how does X pay", "is X in demand", "has X's pay changed over the years", or wants a salary/trend report for a role, category, or city.
---

# jobs-market

Turn a natural-language market question into the right `sjctl market` call(s) and
present the result. The public API exposes two distinct market endpoints — this
skill's job is picking the right one from how the question is phrased.

## Running sjctl

Resolve the `sjctl` binary in this order and use the first that works:

1. `sjctl` on `PATH`
2. `~/.solid-jobs-skills/bin/sjctl` (`sjctl.exe` on Windows) — where the installer puts it
3. `./sjctl` / `./sjctl.exe` in the current repo (local dev)
4. If none exist, install it, then use the path the installer prints on stdout:
   - macOS/Linux: `curl -fsSL https://raw.githubusercontent.com/solid-company/solid-jobs-skills/v0.5.0/scripts/install-sjctl.sh | bash`
   - Windows: `irm https://raw.githubusercontent.com/solid-company/solid-jobs-skills/v0.5.0/scripts/install-sjctl.ps1 | iex`
   - Dev fallback (Go installed, inside the repo): `go run ./cmd/sjctl`

Always pass `--json` so you can parse and reason over the results, then summarize
for the user. Both endpoints reflect the whole live market — they need no prior
search and are independent of the local offer cache used by `stats`.

## Two endpoints, one decision

| Question shape | Command | What you get |
|---|---|---|
| Current/typical pay, demand, "right now" for a division, category, subcategory, subcategory group, or city | `sjctl market <scopeKind> <scopeKey> --json` | A flat live snapshot: `demand`, `salary` (percentile band + B2B/permanent), `experience`, `topLocations`, `topSkills` |
| "Over the years", "trend", "has X changed", "last N years", a single **role or skill name** (e.g. "Golang", "ManualTester") | `sjctl market raport <role> --json` | A `years[]` array, oldest→newest, up to 3 calendar years: offer volume, contract-type split, seniority split, B2B/UoP salary bands, top skills — one entry per year, **plus a nested `quarters[]` array per year** (Q1→Q4, same breakdown minus top skills) for quarter-level questions |
| Both in one ask (e.g. "is Go pay rising, and what's typical now?") | Call both | Present the current snapshot and the yearly trend together |

Key differences to keep straight:

- **scopeKind matters for the first, not the second.** `market <scopeKind> <scopeKey>`
  needs a `scopeKind` ∈ {division, mainCategory, subcategory, subcategoryGroup, city}.
  `market raport <role>` takes only a role/skill name as `scopeKey` — pass `raport`
  itself as the first argument, never a scopeKind.
- **`--fields` only works on the scope endpoint.** It filters
  `demand,salary,experience,topLocations,topSkills`. Passing `--fields` with
  `market raport` is rejected — the raport response is always returned whole.
- **Denominators are local, not global**, in the raport response: each year's
  (and each quarter's) `contractType.total` and `seniority.total` are
  independent of `offerCount` and of each other — don't assume any of them
  sum to the year's/quarter's total offers. `salaryB2B`/`salaryUoP` can be
  `null` — or the key entirely absent — for a year or quarter with no
  matching data.
- **The raport is quarterly under the hood.** Each `years[]` entry carries a
  `quarters[]` array (Q1→Q4, oldest first) with the same fields as the year
  itself except `topSkills` (year-only). The current, still-in-progress year
  may have fewer than 4 quarters.

## Examples

```
sjctl market subcategory React --json                  # current snapshot for a specialization
sjctl market division IT --fields salary,demand --json # narrowed snapshot
sjctl market city warszawa --json                       # a whole city (no topLocations)
sjctl market raport Golang --json                        # 3-year trend for a role
sjctl market raport ManualTester --json
```

## Presenting results

**Snapshot** (`market <scopeKind> <scopeKey>`): lead with the salary band
(min/p25/median/p75/max) and demand (`activeOffers`, `remotePercentage`), then
note the top few `topSkills`/`topLocations` if relevant to the question.

**Raport** (`market raport <role>`): render a compact year-over-year table —
year, offer count, seniority split (junior/regular/senior %), and the B2B
regular/senior median band — since that's what "has pay changed" questions
need. Only expand into contract-type or top-skills detail per year if asked.
Skip years with all-zero `contractType`/`seniority` totals in the narrative
(they mean no usable breakdown that year, not zero offers — `offerCount` is
the real volume figure) but still show the row.

For a quarter-specific question ("how was Q3", "last quarter", "kwartał"),
drill into that year's `quarters[]` array instead of stopping at the yearly
row — same fields as the year (minus `topSkills`), one entry per quarter.
Don't unpack every quarter of every year by default; only go quarterly when
the question asks for that resolution or when a yearly figure looks like it
needs the finer trend to explain (e.g. a mid-year swing).

## Notes

- The API rate limit is 300 req/min; don't loop market calls needlessly — one
  scope call and/or one raport call per question is normally enough.
- Role names for `raport` are free text matched against the API's data (e.g.
  `Golang`, `React`, `ManualTester`). Two distinct "no data" outcomes, don't
  conflate them:
  - **Unrecognized role name** — `sjctl` exits non-zero with an API error
    (the endpoint 404s). Tell the user the role name wasn't recognized and
    suggest a close match (e.g. category/subcategory name from `jobs-search`).
  - **Recognized role, no history** — `sjctl` exits 0 with an empty `years`
    array (human output prints "no raport data for this role"). Tell the user
    there's no yearly data for that role rather than treating it as a search
    failure.
- A salary band with `salaryRangeCount: 0` means no data for that
  seniority/contract-type that year — all-zero, not literally "PLN 0". Never
  report a zero band as a real salary figure.
