package db

import (
	"context"
	"errors"
)

func normalizeCacheMode(value string) string {
	switch value {
	case "off", "on-demand", "precache":
		return value
	}
	return "on-demand"
}
func validCacheMode(value string) bool {
	return value == "off" || value == "on-demand" || value == "precache"
}

func (d *DB) SetCatalogCacheMode(ctx context.Context, id, mode string) (CatalogSource, error) {
	if !validCacheMode(mode) {
		return CatalogSource{}, errors.New("invalid cache mode")
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return CatalogSource{}, err
	}
	defer tx.Rollback(ctx)
	source, err := scanCatalogSource(tx.QueryRow(ctx,
		"UPDATE source_configs SET config=jsonb_set(config,'{cacheMode}',to_jsonb($2::text),true) "+
			"WHERE type='stremio_catalog' AND id=$1::uuid "+
			"RETURNING id::text,name,enabled,config", id, mode))
	if err != nil {
		return CatalogSource{}, err
	}
	if mode == "off" {
		if _, err = tx.Exec(ctx, "DELETE FROM catalog_snapshots WHERE source_id=$1::uuid", id); err != nil {
			return CatalogSource{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return CatalogSource{}, err
	}
	return source, nil
}

func (d *DB) SetAllCatalogCacheModes(ctx context.Context, mode string) error {
	if !validCacheMode(mode) {
		return errors.New("invalid cache mode")
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, "UPDATE source_configs SET config=jsonb_set(config,'{cacheMode}',to_jsonb($1::text),true) "+
		"WHERE type='stremio_catalog'", mode)
	if err != nil {
		return err
	}
	if mode == "off" {
		if _, err = tx.Exec(ctx, "DELETE FROM catalog_snapshots"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
