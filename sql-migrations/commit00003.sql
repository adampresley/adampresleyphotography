-- Add favorites table
CREATE TABLE IF NOT EXISTS "favorites" (
   client_id integer,
   album_id integer,
   image_path text,
   PRIMARY KEY(client_id, album_id, image_path)
);
