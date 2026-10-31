package db

import (
	"context"
	"encoding/json"
)

type CachedTitleSearch struct {
	SourceID string          `json:"sourceId"`
	Catalog  string          `json:"catalog"`
	Item     json.RawMessage `json:"item"`
}

// SearchCachedTitles searches metadata snapshots, not provider streaming URLs.
// It is intentionally limited and does not index user-private watch history.
func (d *DB) SearchCachedTitles(ctx context.Context, phrase string) ([]CachedTitleSearch, error) {
	rows, err := d.pool.Query(ctx, `
 SELECT DISTINCT ON (element.data->>'id',element.data->>'type')
   snap.source_id::text,snap.catalog_id,element.data::text
 FROM catalog_snapshots snap
 JOIN source_configs source ON source.id=snap.source_id
 CROSS JOIN LATERAL jsonb_array_elements(snap.payload) AS element(data)
 WHERE source.enabled
   AND source.type='stremio_catalog'
   AND snap.refreshed_at>now()-interval '48 hours'
   AND strpos(lower(element.data->>'name'),lower($1::text))>0
   AND jsonb_typeof(element.data)='object'
 ORDER BY element.data->>'id',element.data->>'type',snap.refreshed_at DESC
 LIMIT 48`, phrase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]CachedTitleSearch, 0, 32)
	for rows.Next() {
		var row CachedTitleSearch
		if err := rows.Scan(&row.SourceID, &row.Catalog, &row.Item); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}
