package db

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

type StreamAddon struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	ManifestURL string `json:"-"`
}

func readStreamAddon(row pgx.Row) (StreamAddon, error) {
	var s StreamAddon
	var value []byte
	err := row.Scan(&s.ID, &s.Name, &s.Enabled, &value)
	if err != nil {
		return s, err
	}
	var config struct {
		ManifestURL string `json:"manifestUrl"`
	}
	if err = json.Unmarshal(value, &config); err != nil || config.ManifestURL == "" {
		return s, errors.New("invalid stream addon configuration")
	}
	s.ManifestURL = config.ManifestURL
	return s, nil
}
func (d *DB) AddStreamAddon(ctx context.Context, name, manifestURL string) (StreamAddon, error) {
	raw, err := json.Marshal(map[string]string{"manifestUrl": manifestURL})
	if err != nil {
		return StreamAddon{}, err
	}
	s, err := readStreamAddon(d.pool.QueryRow(ctx,
		`INSERT INTO source_configs(name,type,enabled,config) VALUES($1,'stremio_stream',true,$2::jsonb)
    RETURNING id::text,name,enabled,config`, name, string(raw)))
	return s, err
}
func (d *DB) ListStreamAddons(ctx context.Context) ([]StreamAddon, error) {
	rows, err := d.pool.Query(ctx, `SELECT id::text,name,enabled,config FROM source_configs
  WHERE type='stremio_stream' ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]StreamAddon, 0)
	for rows.Next() {
		var s StreamAddon
		var data []byte
		if err = rows.Scan(&s.ID, &s.Name, &s.Enabled, &data); err != nil {
			return nil, err
		}
		var v struct {
			ManifestURL string `json:"manifestUrl"`
		}
		if err = json.Unmarshal(data, &v); err != nil || v.ManifestURL == "" {
			return nil, errors.New("invalid stream addon config")
		}
		s.ManifestURL = v.ManifestURL
		out = append(out, s)
	}
	return out, rows.Err()
}
func (d *DB) StreamAddon(ctx context.Context, id string) (StreamAddon, error) {
	return readStreamAddon(d.pool.QueryRow(ctx, `SELECT id::text,name,enabled,config FROM source_configs
  WHERE type='stremio_stream' AND id=$1::uuid`, id))
}
func (d *DB) DeleteStreamAddon(ctx context.Context, id string) (bool, error) {
	result, err := d.pool.Exec(ctx, `DELETE FROM source_configs WHERE type='stremio_stream' AND id=$1::uuid`, id)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() != 0, nil
}
