package db

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

type StoredCatalog struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}
type SourceConfig struct {
	ManifestURL string          `json:"manifestUrl"`
	Catalogs    []StoredCatalog `json:"catalogs"`
	CacheMode   string          `json:"cacheMode,omitempty"`
}
type CatalogSource struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Enabled     bool            `json:"enabled"`
	Catalogs    []StoredCatalog `json:"catalogs"`
	ManifestURL string          `json:"-"`
	CacheMode   string          `json:"cacheMode"`
}

func (d *DB) AddCatalogSource(ctx context.Context, name, url string, catalogs []StoredCatalog) (CatalogSource, error) {
	var src CatalogSource
	cfg, err := json.Marshal(SourceConfig{ManifestURL: url, Catalogs: catalogs, CacheMode: "on-demand"})
	if err != nil {
		return src, err
	}
	err = d.pool.QueryRow(ctx, `INSERT INTO source_configs(name,type,enabled,config)
  VALUES($1,'stremio_catalog',true,$2::jsonb)
  RETURNING id::text,name,enabled`, name, string(cfg)).
		Scan(&src.ID, &src.Name, &src.Enabled)
	if err != nil {
		return src, err
	}
	src.Catalogs = catalogs
	src.ManifestURL = url
	src.CacheMode = "on-demand"
	return src, nil
}
func scanCatalogSource(row pgx.Row) (CatalogSource, error) {
	var s CatalogSource
	var config []byte
	err := row.Scan(&s.ID, &s.Name, &s.Enabled, &config)
	if err != nil {
		return s, err
	}
	var cfg SourceConfig
	if err = json.Unmarshal(config, &cfg); err != nil {
		return s, errors.New("invalid saved catalog settings")
	}
	s.Catalogs = cfg.Catalogs
	s.ManifestURL = cfg.ManifestURL
	s.CacheMode = normalizeCacheMode(cfg.CacheMode)
	return s, nil
}
func (d *DB) GetCatalogSource(ctx context.Context, id string) (CatalogSource, error) {
	return scanCatalogSource(d.pool.QueryRow(ctx, `SELECT id::text,name,enabled,config FROM source_configs WHERE type='stremio_catalog' AND id=$1::uuid`, id))
}
func (d *DB) ListCatalogSources(ctx context.Context) ([]CatalogSource, error) {
	rows, err := d.pool.Query(ctx, `SELECT id::text,name,enabled,config FROM source_configs WHERE type='stremio_catalog' ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CatalogSource, 0)
	for rows.Next() {
		var s CatalogSource
		var cfg []byte
		if err = rows.Scan(&s.ID, &s.Name, &s.Enabled, &cfg); err != nil {
			return nil, err
		}
		var v SourceConfig
		if err = json.Unmarshal(cfg, &v); err != nil {
			return nil, errors.New("invalid saved catalog settings")
		}
		s.Catalogs = v.Catalogs
		s.ManifestURL = v.ManifestURL
		s.CacheMode = normalizeCacheMode(v.CacheMode)
		out = append(out, s)
	}
	return out, rows.Err()
}
func (d *DB) DeleteCatalogSource(ctx context.Context, id string) (bool, error) {
	r, err := d.pool.Exec(ctx, `DELETE FROM source_configs WHERE type='stremio_catalog' AND id=$1::uuid`, id)
	if err != nil {
		return false, err
	}
	return r.RowsAffected() != 0, nil
}
