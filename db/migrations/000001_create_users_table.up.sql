BEGIN;

CREATE TABLE IF NOT EXISTS chat_user(
  chat_user_id serial PRIMARY KEY,
  username VARCHAR (50) UNIQUE NOT NULL,
  password VARCHAR (50) NOT NULL
);

COMMIT;
