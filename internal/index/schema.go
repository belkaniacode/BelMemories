package index

const schemaVersion = "1"

const schemaSQL = `
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS files (
	hash        BLOB PRIMARY KEY,
	size        INTEGER NOT NULL,
	rel_path    TEXT NOT NULL UNIQUE,
	kind        TEXT NOT NULL,
	category    TEXT NOT NULL,
	year        INTEGER NOT NULL,
	date_source TEXT NOT NULL,
	orig_name   TEXT NOT NULL,
	added_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS files_size ON files(size);
CREATE TABLE IF NOT EXISTS runs (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	started_at  INTEGER NOT NULL,
	finished_at INTEGER,
	sources     TEXT NOT NULL,
	status      TEXT NOT NULL DEFAULT 'running',
	stats_json  TEXT
);
CREATE TABLE IF NOT EXISTS journal (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id       INTEGER NOT NULL,
	src_path     TEXT NOT NULL,
	tmp_path     TEXT NOT NULL,
	dst_rel_path TEXT,
	hash         BLOB,
	state        TEXT NOT NULL,
	updated_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS journal_state ON journal(state);
`
