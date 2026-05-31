-- Drop the FK constraint first, then change the column type.
-- We no longer store users locally — Clerk owns user identity.
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_user_id_fkey;
ALTER TABLE projects ALTER COLUMN user_id TYPE TEXT;