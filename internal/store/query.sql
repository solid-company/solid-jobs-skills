-- Queries for solid-jobs-skills, compiled by sqlc into internal/store/sqlcgen.
-- The store package wraps these generated methods; no SQL is inlined in Go.

-- ===== offers =====

-- name: UpsertOffer :exec
INSERT INTO offers (offer_key, title, company, division, category, sub_category,
    experience_level, salary_from, salary_to, currency, is_remote, is_hybrid,
    valid_from, valid_to, url, locations, skills, raw, fetched_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(offer_key) DO UPDATE SET
    title=excluded.title, company=excluded.company, division=excluded.division,
    category=excluded.category, sub_category=excluded.sub_category,
    experience_level=excluded.experience_level, salary_from=excluded.salary_from,
    salary_to=excluded.salary_to, currency=excluded.currency,
    is_remote=excluded.is_remote, is_hybrid=excluded.is_hybrid,
    valid_from=excluded.valid_from, valid_to=excluded.valid_to,
    url=excluded.url, locations=excluded.locations, skills=excluded.skills,
    raw=excluded.raw, fetched_at=excluded.fetched_at;

-- name: GetOfferRaw :one
SELECT raw FROM offers WHERE offer_key = ?;

-- name: OfferExists :one
SELECT EXISTS(SELECT 1 FROM offers WHERE offer_key = ?);

-- name: SalaryRows :many
SELECT salary_from, salary_to, currency, is_remote, experience_level, division
FROM offers
WHERE (CAST(sqlc.narg('division') AS TEXT) IS NULL OR division = sqlc.narg('division'))
  AND (CAST(sqlc.narg('remote') AS INTEGER) IS NULL OR is_remote = sqlc.narg('remote'))
  AND (CAST(sqlc.narg('skill') AS TEXT) IS NULL
       OR title LIKE sqlc.narg('skill')
       OR category LIKE sqlc.narg('skill')
       OR sub_category LIKE sqlc.narg('skill')
       OR skills LIKE sqlc.narg('skill'))
  AND (CAST(sqlc.narg('city') AS TEXT) IS NULL OR locations LIKE sqlc.narg('city'));

-- ===== profiles =====

-- name: InsertProfile :one
INSERT INTO profiles (name, content, is_default, created_at, updated_at)
VALUES (?, ?, 0, ?, ?)
RETURNING id;

-- name: CountProfiles :one
SELECT COUNT(*) FROM profiles;

-- name: SeedDefaultProfile :exec
INSERT INTO profiles (name, content, is_default, created_at, updated_at)
VALUES (?, '', 1, ?, ?);

-- name: SetProfileContent :execrows
UPDATE profiles SET content = ?, updated_at = ? WHERE id = ?;

-- name: SetDefaultProfile :exec
UPDATE profiles SET is_default = (id = ?);

-- name: ProfileExists :one
SELECT EXISTS(SELECT 1 FROM profiles WHERE id = ?);

-- name: GetProfile :one
SELECT id, name, content, is_default, created_at, updated_at
FROM profiles WHERE id = ?;

-- name: GetProfileByName :one
SELECT id, name, content, is_default, created_at, updated_at
FROM profiles WHERE name = ?;

-- name: DefaultProfile :one
SELECT id, name, content, is_default, created_at, updated_at
FROM profiles WHERE is_default = 1 LIMIT 1;

-- name: ListProfiles :many
SELECT id, name, content, is_default, created_at, updated_at
FROM profiles ORDER BY is_default DESC, name ASC;

-- ===== tracker =====

-- name: InsertTracked :exec
INSERT INTO tracker (offer_key, profile_id, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(offer_key, profile_id) DO NOTHING;

-- name: SetStatus :execrows
UPDATE tracker SET status = ?, updated_at = ?
WHERE offer_key = ? AND profile_id = ?;

-- name: SetNotes :execrows
UPDATE tracker SET notes = ?, updated_at = ?
WHERE offer_key = ? AND profile_id = ?;

-- name: RemoveTracked :execrows
DELETE FROM tracker WHERE offer_key = ? AND profile_id = ?;

-- name: ExpireStale :execrows
UPDATE tracker SET status = 'expired', updated_at = sqlc.arg('updated_at')
WHERE profile_id = sqlc.arg('profile_id')
  AND status NOT IN ('expired', 'offer', 'rejected')
  AND offer_key IN (
      SELECT offer_key FROM offers
      WHERE valid_to != '' AND valid_to < sqlc.arg('cutoff'));

-- name: ListTracked :many
SELECT t.offer_key, t.profile_id, t.status, t.notes,
       o.title, o.company, o.url, o.valid_to,
       COALESCE((SELECT e.grade FROM evaluations e
                 WHERE e.offer_key = t.offer_key AND e.profile_id = t.profile_id
                 ORDER BY e.created_at DESC LIMIT 1), '') AS grade,
       t.created_at, t.updated_at
FROM tracker t
JOIN offers o ON o.offer_key = t.offer_key
WHERE t.profile_id = sqlc.arg('profile_id')
  AND (CAST(sqlc.narg('status') AS TEXT) IS NULL OR t.status = sqlc.narg('status'))
ORDER BY t.updated_at DESC;

-- name: GetTracked :one
SELECT t.offer_key, t.profile_id, t.status, t.notes,
       o.title, o.company, o.url, o.valid_to,
       t.created_at, t.updated_at
FROM tracker t
JOIN offers o ON o.offer_key = t.offer_key
WHERE t.offer_key = ? AND t.profile_id = ?;

-- ===== evaluations =====

-- name: InsertEvaluation :one
INSERT INTO evaluations (offer_key, profile_id, grade, dimensions, rationale, created_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, created_at;

-- name: LatestEvaluation :one
SELECT id, offer_key, profile_id, grade, dimensions, rationale, created_at
FROM evaluations
WHERE offer_key = ? AND profile_id = ?
ORDER BY created_at DESC, id DESC LIMIT 1;

-- ===== interviews =====

-- name: InsertInterviewPrep :one
INSERT INTO interview_preps (offer_key, profile_id, readiness, gaps, questions_to_ask, summary, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, created_at, updated_at;

-- name: InsertInterviewQuestion :one
INSERT INTO interview_questions (prep_id, category, question, difficulty, talking_points, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: LatestInterviewPrep :one
SELECT id, offer_key, profile_id, readiness, gaps, questions_to_ask, summary, created_at, updated_at
FROM interview_preps
WHERE offer_key = ? AND profile_id = ?
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: ListInterviewPreps :many
-- Latest prep per offer for a profile (history is preserved, but the list shows
-- one row per offer), decorated with the offer's title/company/url.
SELECT p.id, p.offer_key, p.profile_id, p.readiness, p.summary,
       p.created_at, p.updated_at,
       o.title, o.company, o.url
FROM interview_preps p
JOIN offers o ON o.offer_key = p.offer_key
WHERE p.profile_id = ?
  AND p.id = (
      SELECT p2.id FROM interview_preps p2
      WHERE p2.offer_key = p.offer_key AND p2.profile_id = p.profile_id
      ORDER BY p2.created_at DESC, p2.id DESC LIMIT 1)
ORDER BY p.updated_at DESC, p.id DESC;

-- name: QuestionsForPrep :many
SELECT id, prep_id, category, question, difficulty, talking_points, confidence, practiced_count, created_at, updated_at
FROM interview_questions
WHERE prep_id = ?
ORDER BY category, id;

-- name: NextPracticeQuestions :many
SELECT id, prep_id, category, question, difficulty, talking_points, confidence, practiced_count, created_at, updated_at
FROM interview_questions
WHERE prep_id = ?
ORDER BY confidence ASC, practiced_count ASC, id ASC
LIMIT ?;

-- name: GetQuestion :one
SELECT id, prep_id, category, question, difficulty, talking_points, confidence, practiced_count, created_at, updated_at
FROM interview_questions
WHERE id = ?;

-- name: UpdateQuestionConfidence :execrows
UPDATE interview_questions
SET confidence = ?, practiced_count = practiced_count + 1, updated_at = ?
WHERE id = ?;

-- name: UpdatePrepReadiness :execrows
UPDATE interview_preps SET readiness = ?, updated_at = ? WHERE id = ?;

-- name: GetInterviewPrep :one
SELECT id, offer_key, profile_id, readiness, gaps, questions_to_ask, summary, created_at, updated_at
FROM interview_preps
WHERE id = ?;

-- ===== watches =====

-- name: UpsertWatch :exec
INSERT INTO watches (name, division, params, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET division = excluded.division, params = excluded.params;

-- name: GetWatchByName :one
SELECT id, name, division, params, created_at FROM watches WHERE name = ?;

-- name: ListWatches :many
SELECT id, name, division, params, created_at FROM watches ORDER BY name ASC;

-- name: RemoveWatch :execrows
DELETE FROM watches WHERE name = ?;

-- ===== seen =====

-- name: MarkSeen :execrows
INSERT INTO seen (offer_key, seen_at) VALUES (?, ?)
ON CONFLICT(offer_key) DO NOTHING;

-- name: IsSeen :one
SELECT EXISTS(SELECT 1 FROM seen WHERE offer_key = ?);

-- name: SeenKeys :many
SELECT offer_key FROM seen WHERE offer_key IN (sqlc.slice('keys'));
