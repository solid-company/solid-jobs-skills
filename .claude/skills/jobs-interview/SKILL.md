---
name: jobs-interview
description: Prepare the user for an interview based on a cached job offer and their profile — gap analysis with a readiness score, a tailored question bank (technical per required skill + behavioral/situational with talking points), questions to ask the recruiter, and a mock-interview mode that scores confidence. Use when the user asks "prepare me for this interview", "interview questions for this job", "mock interview", or "przygotuj mnie do rozmowy".
---

# jobs-interview

Turn a cached offer + the user's profile into an interview prep pack, persist it, and optionally run a mock interview that tracks readiness.

## Inputs

1. The offer — read it from the cache by key with `sjctl offer show <offerKey> --json` (full detail, description as plain text, no API call). It must have been seen by a prior search; if not, run `/jobs-search` first. Don't re-run `search` to fetch one offer.
2. The profile — `sjctl profile show --json` (the default profile, or pass `--profile <name>`).
3. The latest evaluation, as a seed for the gap analysis — `sjctl evaluate show <offerKey> --json`. If it exists, reuse `dimensions.skillMatch` / `seniorityFit` and `rationale`, and seed readiness from `grade`. If it does not exist, do the gap analysis yourself and suggest running `/jobs-evaluate` first.

Resolve `sjctl` in this order: (1) on `PATH`, (2) `~/.solid-jobs-skills/bin/sjctl[.exe]`, (3) `./sjctl[.exe]` in the repo. If none exist, install it and use the printed path:
- macOS/Linux: `curl -fsSL https://raw.githubusercontent.com/solid-company/solid-jobs-skills/v0.3.0/scripts/install-sjctl.sh | bash`
- Windows: `irm https://raw.githubusercontent.com/solid-company/solid-jobs-skills/v0.3.0/scripts/install-sjctl.ps1 | iex`
- Dev fallback (Go, inside repo): `go run ./cmd/sjctl`

## Build the prep pack

1. **Gap analysis + readiness.** Compare the offer's required skills (`offer.skills`, each with a `level`) and `experienceLevel` against the profile's strong/working skills and seniority. Reuse the evaluation seed when present. Produce:
   - `gaps`: the weak spots, each `{skill, severity (low|medium|high), note}`.
   - a `readiness` score 0–100 (dry-run estimate; the mock loop recomputes it from confidence).
   - a short `summary` naming the biggest strengths and the biggest risk.
2. **Question bank.** Generate questions the interviewer is likely to ask, each `{category, question, difficulty (easy|medium|hard), talkingPoints}`:
   - `technical` — at least one per required skill, difficulty scaled to `experienceLevel`.
   - `behavioral` / `situational` — mapped to the profile's real experience so `talkingPoints` cite concrete stories the candidate can tell (STAR).
   - `company` — role/company/domain specific where the description gives signal.
3. **Questions to ask the recruiter.** 3–6 sharp questions tailored to the company/role (`questionsToAsk`).

Persist everything (JSON is single-quoted for the shell):

```
sjctl interview save <offerKey> \
  --readiness 60 \
  --summary "Strong Go; gap in Kubernetes and event-driven design." \
  --gaps '[{"skill":"Kubernetes","severity":"high","note":"no production experience"}]' \
  --ask '["How is on-call handled?","What does success look like in 6 months?"]' \
  --questions '[{"category":"technical","question":"Explain a goroutine leak and how you would find one","difficulty":"medium","talkingPoints":"unbuffered channels, missing context cancel, pprof"}]'
```

## Mock interview

Run this when the user wants to practise:

1. Pull the next questions to drill (lowest confidence first): `sjctl interview practice <offerKey> --json` (add `--limit N`).
2. Ask **one question at a time**. Let the user answer, then evaluate it against the question's `talkingPoints`: give concrete feedback and a model answer.
3. Record how solid the answer was: `sjctl interview rate <questionId> <0-5>`. Readiness is recomputed automatically from the mean confidence.
4. After the session, run `sjctl interview show <offerKey>` and report the new readiness and the questions still worth drilling.

In `interview show --json`, the recruiter questions saved via `--ask` come back under the `questionsToAsk` field (not `ask`), the gap list under `gaps`, and the bank under `questions`.

## Output to the user

Lead with the readiness score and the single biggest gap, then the strengths, a sample of the question bank grouped by category, and the questions to ask the recruiter. Offer to start a mock interview. Use `sjctl interview list` to show readiness across all prepped offers.
