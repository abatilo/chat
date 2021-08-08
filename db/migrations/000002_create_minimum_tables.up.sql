BEGIN;

  CREATE TABLE IF NOT EXISTS chat_user(
    id bigserial PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL CONSTRAINT password_check CHECK (char_length(password) <= 72)
  );

  CREATE TABLE IF NOT EXISTS message_type(
    id smallserial PRIMARY KEY,
    name TEXT UNIQUE NOT NULL
  );

  INSERT INTO message_type(name) VALUES ('text'), ('image'), ('video');

  CREATE TABLE IF NOT EXISTS message(
    id bigserial PRIMARY KEY,
    sender_id bigint NOT NULL REFERENCES chat_user(id) ON UPDATE CASCADE,
    recipient_id bigint NOT NULL REFERENCES chat_user(id) ON UPDATE CASCADE,
    message_type_id smallint NOT NULL REFERENCES message_type(id) ON UPDATE CASCADE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
  );

  CREATE INDEX message_recipient_id_created_at_idx ON message (recipient_id, created_at);

  CREATE TABLE IF NOT EXISTS text_message(
    id bigserial PRIMARY KEY,
    message_id bigint NOT NULL REFERENCES message(id) ON UPDATE CASCADE,
    text TEXT NOT NULL
  );

  CREATE TABLE IF NOT EXISTS image_message(
    id bigserial PRIMARY KEY,
    message_id bigint NOT NULL REFERENCES message(id) ON UPDATE CASCADE,
    url TEXT NOT NULL,
    width smallint NOT NULL DEFAULT 64,
    height smallint NOT NULL DEFAULT 64
  );

  CREATE TABLE IF NOT EXISTS video_source(
    id smallserial PRIMARY KEY,
    name TEXT NOT NULL
  );

  INSERT INTO video_source(id, name) VALUES (1, 'youtube');

  CREATE TABLE IF NOT EXISTS video_message(
    id bigserial PRIMARY KEY,
    message_id bigint NOT NULL REFERENCES message(id) ON UPDATE CASCADE,
    url TEXT NOT NULL,
    source smallint NOT NULL DEFAULT 1 REFERENCES video_source(id) ON UPDATE CASCADE
  );

COMMIT;
