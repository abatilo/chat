BEGIN;

CREATE TABLE IF NOT EXISTS chat_user(
  chat_user_id bigserial PRIMARY KEY,
  username VARCHAR (32) UNIQUE NOT NULL,
  password VARCHAR (60) NOT NULL
);

COMMIT;
