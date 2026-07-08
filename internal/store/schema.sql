-- Schema for solid-jobs-skills. Applied idempotently on every Open via
-- CREATE TABLE IF NOT EXISTS; safe to re-run.

-- Cached job offers. The full API object is kept in `raw` (JSON) so callers can
-- reconstruct the typed Offer; indexed columns support filtering and stats.
CREATE TABLE IF NOT EXISTS offers (
    offer_key        TEXT PRIMARY KEY,
    title            TEXT NOT NULL,
    company          TEXT NOT NULL,
    division         TEXT NOT NULL,
    category         TEXT,
    sub_category     TEXT,
    experience_level TEXT,
    salary_from      INTEGER,
    salary_to        INTEGER,
    currency         TEXT,
    is_remote        INTEGER NOT NULL DEFAULT 0,
    is_hybrid        INTEGER NOT NULL DEFAULT 0,
    valid_from       TEXT,
    valid_to         TEXT,
    url              TEXT,
    locations        TEXT,  -- newline-joined offer.locations, for city filtering
    skills           TEXT,  -- newline-joined offer.skills names, for skill filtering
    raw              TEXT NOT NULL,
    fetched_at       TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_offers_division ON offers(division);
CREATE INDEX IF NOT EXISTS idx_offers_salary ON offers(salary_from, salary_to);
CREATE INDEX IF NOT EXISTS idx_offers_valid_to ON offers(valid_to);

-- User profiles. Exactly one row should have is_default = 1.
CREATE TABLE IF NOT EXISTS profiles (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    content    TEXT NOT NULL DEFAULT '',
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Application pipeline. One row per (offer, profile) being tracked.
CREATE TABLE IF NOT EXISTS tracker (
    offer_key  TEXT NOT NULL,
    profile_id INTEGER NOT NULL,
    status     TEXT NOT NULL DEFAULT 'saved',
    notes      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (offer_key, profile_id),
    FOREIGN KEY (offer_key) REFERENCES offers(offer_key) ON DELETE CASCADE,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

-- AI evaluations. Kept separate from tracker so an offer can be re-evaluated
-- (history) and compared across profiles.
CREATE TABLE IF NOT EXISTS evaluations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    offer_key  TEXT NOT NULL,
    profile_id INTEGER NOT NULL,
    grade      TEXT NOT NULL,
    dimensions TEXT NOT NULL DEFAULT '{}', -- JSON object of dimension -> score
    rationale  TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    FOREIGN KEY (offer_key) REFERENCES offers(offer_key) ON DELETE CASCADE,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_eval_offer ON evaluations(offer_key, profile_id);

-- Interview preparation sessions. One row per prep of an (offer, profile);
-- history is preserved (like evaluations) so an offer can be re-prepped. The
-- readiness score is recomputed from the average question confidence.
CREATE TABLE IF NOT EXISTS interview_preps (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    offer_key        TEXT NOT NULL,
    profile_id       INTEGER NOT NULL,
    readiness        INTEGER NOT NULL DEFAULT 0,   -- 0..100
    gaps             TEXT NOT NULL DEFAULT '[]',   -- JSON: [{skill, severity, note}]
    questions_to_ask TEXT NOT NULL DEFAULT '[]',   -- JSON array of strings
    summary          TEXT NOT NULL DEFAULT '',     -- gap-analysis narrative / strengths
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    FOREIGN KEY (offer_key) REFERENCES offers(offer_key) ON DELETE CASCADE,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_prep_offer ON interview_preps(offer_key, profile_id);

-- Individual interview questions belonging to a prep. Drives the mock-interview
-- loop: confidence (0..5) is self-rated after practising and feeds readiness.
CREATE TABLE IF NOT EXISTS interview_questions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    prep_id         INTEGER NOT NULL,
    category        TEXT NOT NULL,                 -- technical | behavioral | situational | company
    question        TEXT NOT NULL,
    difficulty      TEXT NOT NULL DEFAULT 'medium',-- easy | medium | hard
    talking_points  TEXT NOT NULL DEFAULT '',
    confidence      INTEGER NOT NULL DEFAULT 0,    -- 0..5 (0 = not yet practised)
    practiced_count INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    FOREIGN KEY (prep_id) REFERENCES interview_preps(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_q_prep ON interview_questions(prep_id);

-- Offer keys already reported by `sync`, so a new run only surfaces new offers.
-- NOTE: this table grows unbounded — there's no pruning of old keys. Fine for a
-- personal tool (one row per offer ever seen), but if it ever matters, prune by
-- seen_at (the column exists for exactly this) or join against live offers.
CREATE TABLE IF NOT EXISTS seen (
    offer_key TEXT PRIMARY KEY,
    seen_at   TEXT NOT NULL
);

-- Saved searches driving `sync` / digest. params is a JSON-encoded SearchParams
-- plus the division.
CREATE TABLE IF NOT EXISTS watches (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    division   TEXT NOT NULL,
    params     TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);
