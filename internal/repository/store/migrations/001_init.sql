-- 版 1: connections・settings・snapshots・notified_states（docs/architecture/er.md §3）。
-- 既存のテーブルと衝突したら失敗させる（別アプリの DB を黙って流用しない）。

CREATE TABLE connections (
    id           TEXT NOT NULL PRIMARY KEY,
    name         TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    url          TEXT NOT NULL UNIQUE,
    last_session TEXT,
    baselined_at TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

CREATE TABLE settings (
    id                    INTEGER NOT NULL PRIMARY KEY CHECK (id = 1),
    notifications_enabled INTEGER NOT NULL DEFAULT 1 CHECK (notifications_enabled IN (0, 1)),
    screen_poll_seconds   INTEGER NOT NULL DEFAULT 10 CHECK (screen_poll_seconds IN (5, 10, 30, 60)),
    notify_poll_minutes   INTEGER NOT NULL DEFAULT 10 CHECK (notify_poll_minutes IN (5, 10, 30, 60)),
    debug_log             INTEGER NOT NULL DEFAULT 0 CHECK (debug_log IN (0, 1)),
    last_connection_id    TEXT REFERENCES connections (id) ON DELETE SET NULL,
    updated_at            TEXT NOT NULL
);

INSERT INTO settings (id, updated_at) VALUES (1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));

CREATE TABLE snapshots (
    connection_id  TEXT NOT NULL REFERENCES connections (id) ON DELETE CASCADE,
    session_id     TEXT NOT NULL,
    fetched_at     TEXT NOT NULL,
    schema_version INTEGER NOT NULL,
    report_json    TEXT NOT NULL CHECK (json_valid(report_json)),
    PRIMARY KEY (connection_id, session_id)
);

CREATE TABLE notified_states (
    connection_id TEXT NOT NULL REFERENCES connections (id) ON DELETE CASCADE,
    session_id    TEXT NOT NULL,
    pane_id       TEXT NOT NULL,
    state         TEXT NOT NULL CHECK (state IN ('asking', 'stalled')),
    notified_at   TEXT NOT NULL,
    PRIMARY KEY (connection_id, session_id, pane_id, state)
);

CREATE INDEX idx_notified_states_connection_notified_at
    ON notified_states (connection_id, notified_at);
