package db

import (
	"context"
	"errors"
	"time"
)

// Catalog snapshots store metadata only. Source-specific signed playback
// addresses and authentication never enter this table.
func (d *DB) CatalogSnapshot(ctx context.Context, sourceID, typ, catalogID string) ([]byte, time.Time, error) {
	var payload []byte
	var updated time.Time
	err := d.pool.QueryRow(ctx,
		"SELECT payload::text,refreshed_at FROM catalog_snapshots "+
			"WHERE source_id=$1::uuid AND content_type=$2 AND catalog_id=$3",
		sourceID, typ, catalogID).Scan(&payload, &updated)
	return payload, updated, err
}
func (d *DB) SaveCatalogSnapshot(ctx context.Context, sourceID, typ, catalogID string, payload []byte) error {
	if len(payload) == 0 || len(payload) > 1024*1024 {
		return errors.New("catalog snapshot too large")
	}
	_, err := d.pool.Exec(ctx,
		"INSERT INTO catalog_snapshots(source_id,content_type,catalog_id,payload,refreshed_at) "+
			"VALUES($1::uuid,$2,$3,$4::jsonb,now()) "+
			"ON CONFLICT(source_id,content_type,catalog_id) DO UPDATE "+
			"SET payload=excluded.payload,refreshed_at=excluded.refreshed_at",
		sourceID, typ, catalogID, string(payload))
	return err
}
