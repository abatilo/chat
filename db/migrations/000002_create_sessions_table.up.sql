BEGIN;

-- https://github.com/alexedwards/scs/tree/91e3021b78b2f4dccd4c51a5bc293577029765f0/pgxstore#setup
CREATE TABLE sessions (
	token TEXT PRIMARY KEY,
	data BYTEA NOT NULL,
	expiry TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessions_expiry_idx ON sessions (expiry);

COMMIT;
