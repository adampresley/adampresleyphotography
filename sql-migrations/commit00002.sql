-- Add email column to clients table if it doesn't exist
ALTER TABLE clients ADD COLUMN email TEXT;
