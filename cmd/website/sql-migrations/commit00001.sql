--
-- clients
--
CREATE TABLE IF NOT EXISTS "clients" (
  id integer PRIMARY KEY AUTOINCREMENT,
  created_at datetime,
  updated_at datetime,
  deleted_at datetime,
  password text,
  name text,
  UNIQUE (password)
);

CREATE INDEX IF NOT EXISTS idx_clients_deleted_at ON clients (deleted_at);

--
-- albums
--
CREATE TABLE IF NOT EXISTS "albums" (
  id integer PRIMARY KEY AUTOINCREMENT,
  created_at datetime,
  updated_at datetime,
  deleted_at datetime,
  name text,
  "path" text,
  client_id integer,
  shoot_date datetime,
  poster_image_path text
);

CREATE INDEX IF NOT EXISTS idx_albums_deleted_at ON albums (deleted_at);
CREATE INDEX IF NOT EXISTS idx_albums_client_id ON albums (client_id);
