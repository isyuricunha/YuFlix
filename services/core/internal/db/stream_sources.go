package db

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

type StreamAddon struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	ManifestURL  string `json:"-"`
	NetworkRoute string `json:"networkRoute"`
	Priority     int    `json:"priority"`
	TimeoutMs    int    `json:"timeoutMs"`
}

func readStreamAddon(row pgx.Row) (StreamAddon, error) {
	var s StreamAddon
	var value []byte
	err := row.Scan(&s.ID, &s.Name, &s.Enabled, &value)
	if err != nil {
		return s, err
	}
	var config struct {
		ManifestURL  string `json:"manifestUrl"`
		NetworkRoute string `json:"networkRoute"`
		Priority     int    `json:"priority"`
		TimeoutMs    int    `json:"timeoutMs"`
	}
	if err = json.Unmarshal(value, &config); err != nil || config.ManifestURL == "" {
		return s, errors.New("invalid stream addon configuration")
	}
	s.ManifestURL = config.ManifestURL
	s.NetworkRoute = normalizedStreamRoute(config.NetworkRoute)
	s.Priority = normalizedPriority(config.Priority)
	s.TimeoutMs = normalizedTimeout(config.TimeoutMs)
	return s, nil
}
func (d *DB) AddStreamAddon(ctx context.Context, name, manifestURL string) (StreamAddon, error) {
	raw, err := json.Marshal(map[string]any{"manifestUrl": manifestURL, "networkRoute": "direct", "priority": 500, "timeoutMs": 5000})
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
			ManifestURL  string `json:"manifestUrl"`
			NetworkRoute string `json:"networkRoute"`
			Priority     int    `json:"priority"`
			TimeoutMs    int    `json:"timeoutMs"`
		}
		if err = json.Unmarshal(data, &v); err != nil || v.ManifestURL == "" {
			return nil, errors.New("invalid stream addon config")
		}
		s.ManifestURL = v.ManifestURL
		s.NetworkRoute = normalizedStreamRoute(v.NetworkRoute)
		s.Priority = normalizedPriority(v.Priority)
		s.TimeoutMs = normalizedTimeout(v.TimeoutMs)
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

// Existing records without networkRoute use safe direct egress.
func normalizedStreamRoute(route string) string {
	switch route {
	case "direct", "egress", "auto":
		return route
	default:
		return "direct"
	}
}
func (d *DB) UpdateStreamAddonRoute(ctx context.Context, id, route string) (StreamAddon, error) {
	if route != "direct" && route != "egress" && route != "auto" {
		return StreamAddon{}, errors.New("invalid route")
	}
	return readStreamAddon(d.pool.QueryRow(ctx,
		`UPDATE source_configs SET config=jsonb_set(config,'{networkRoute}',to_jsonb($2::text),true)
   WHERE type='stremio_stream' AND id=$1::uuid
   RETURNING id::text,name,enabled,config`, id, route))
}

// Older sources without a policy receive safe defaults without migrations.
func normalizedPriority(value int) int {
	if value < 1 || value > 1000 {
		return 500
	}
	return value
}
func normalizedTimeout(value int) int {
	if value < 1000 || value > 10000 {
		return 5000
	}
	return value
}

// Policies live in existing JSONB config and never expose source credentials.
func (d *DB) UpdateStreamAddonPolicy(ctx context.Context, id string, priority, timeoutMs int) (StreamAddon, error) {
	if priority < 1 || priority > 1000 || timeoutMs < 1000 || timeoutMs > 10000 {
		return StreamAddon{}, errors.New("invalid source policy")
	}
	return readStreamAddon(d.pool.QueryRow(ctx,
		"UPDATE source_configs SET config=jsonb_set("+
			"jsonb_set(config,'{priority}',to_jsonb($2::integer),true),"+
			"'{timeoutMs}',to_jsonb($3::integer),true) "+
			"WHERE type='stremio_stream' AND id=$1::uuid "+
			"RETURNING id::text,name,enabled,config", id, priority, timeoutMs))
}
