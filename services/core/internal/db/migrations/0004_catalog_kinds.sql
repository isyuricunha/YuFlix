-- Add catalog-only resource types without changing existing entries.
ALTER TABLE catalog_snapshots
 DROP CONSTRAINT IF EXISTS catalog_snapshots_content_type_check;
ALTER TABLE catalog_snapshots
 ADD CONSTRAINT catalog_snapshots_content_type_check
 CHECK (content_type IN ('movie','series','anime','collections'));
