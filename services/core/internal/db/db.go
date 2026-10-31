package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrNoDatabase = errors.New("database not configured")
var ErrNoRows = pgx.ErrNoRows

type DB struct{ pool *pgxpool.Pool }

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

type Setting struct {
	Key       string    `json:"key"`
	Value     []byte    `json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func Open(ctx context.Context, dsn string) (*DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, ErrNoDatabase
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid database connection settings")
	}
	cfg.MaxConns = 5
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 10 * time.Minute
	cfg.MaxConnLifetime = 45 * time.Minute
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pool initialization failed: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = p.Ping(pingCtx); err != nil {
		p.Close()
		return nil, errors.New("database connection or authentication failed")
	}
	return &DB{pool: p}, nil
}

func (d *DB) Close() {
	if d != nil {
		d.pool.Close()
	}
}
func (d *DB) Ping(ctx context.Context) error { return d.pool.Ping(ctx) }

// Migrate explicitly runs numbered, checksummed migrations under a transaction-level advisory lock.
// No DDL is applied on normal HTTP startup.
func (d *DB) Migrate(ctx context.Context) error {
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(763846264)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version integer PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		var version int
		if _, err = fmt.Sscanf(file, "migrations/%04d_", &version); err != nil {
			return fmt.Errorf("invalid migration filename %q", file)
		}
		sql, err := migrations.ReadFile(file)
		if err != nil {
			return err
		}
		h := sha256.Sum256(sql)
		checksum := hex.EncodeToString(h[:])
		var previous string
		err = tx.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE version=$1", version).Scan(&previous)
		if err == nil {
			if previous != checksum {
				return fmt.Errorf("migration %d checksum mismatch", version)
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %d failed: %w", version, err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)", version, checksum); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Status is read-only and deliberately excludes connection URLs and credentials.
type Status struct {
	DatabaseIsYuFlix bool
	ServerMajor      int
	Superuser        bool
	PublicTables     int
	SchemaReady      bool
}

func (d *DB) Inspect(ctx context.Context) (Status, error) {
	var status Status
	var serverVersion int
	err := d.pool.QueryRow(ctx, `SELECT current_database() = 'yuflix',
        current_setting('server_version_num')::integer,
        coalesce((SELECT rolsuper FROM pg_roles WHERE rolname=current_user), true),
        (SELECT count(*)::integer FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE')`).
		Scan(&status.DatabaseIsYuFlix, &serverVersion, &status.Superuser, &status.PublicTables)
	if err != nil {
		return Status{}, errors.New("cannot inspect target database")
	}
	status.ServerMajor = serverVersion / 10000
	status.SchemaReady = d.SchemaReady(ctx)
	return status, nil
}

func (d *DB) SchemaReady(ctx context.Context) bool {
	var v int
	return d.pool.QueryRow(ctx, "SELECT max(version) FROM schema_migrations").Scan(&v) == nil && v >= 2
}

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Role, &u.Enabled, &u.CreatedAt)
	return u, err
}
func (d *DB) UserForLogin(ctx context.Context, username string) (User, string, error) {
	var u User
	var hash string
	err := d.pool.QueryRow(ctx, `SELECT id::text,username,role,enabled,created_at,password_hash FROM users WHERE username=$1`, username).
		Scan(&u.ID, &u.Username, &u.Role, &u.Enabled, &u.CreatedAt, &hash)
	return u, hash, err
}
func (d *DB) AddUser(ctx context.Context, username, hash, role string) (User, error) {
	return scanUser(d.pool.QueryRow(ctx, `INSERT INTO users(username,password_hash,role) VALUES($1,$2,$3) RETURNING id::text,username,role,enabled,created_at`, username, hash, role))
}
func (d *DB) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := d.pool.Query(ctx, `SELECT id::text,username,role,enabled,created_at FROM users ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]User, 0)
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.ID, &u.Username, &u.Role, &u.Enabled, &u.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, rows.Err()
}
func (d *DB) UserBySession(ctx context.Context, hash []byte, transport string) (User, error) {
	return scanUser(d.pool.QueryRow(ctx, `SELECT u.id::text,u.username,u.role,u.enabled,u.created_at
		FROM sessions s JOIN users u ON s.user_id=u.id
		WHERE s.token_hash=$1 AND s.transport=$2 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.enabled`, hash, transport))
}
func (d *DB) CreateSession(ctx context.Context, userID string, hash []byte, expires time.Time, transport string) error {
	_, err := d.pool.Exec(ctx, `INSERT INTO sessions (token_hash,user_id,expires_at,transport) VALUES ($1,$2::uuid,$3,$4)`, hash, userID, expires, transport)
	return err
}
func (d *DB) RevokeSession(ctx context.Context, hash []byte) error {
	_, err := d.pool.Exec(ctx, "UPDATE sessions SET revoked_at=now() WHERE token_hash=$1", hash)
	return err
}
func (d *DB) GetSettings(ctx context.Context) (map[string][]byte, error) {
	rows, err := d.pool.Query(ctx, "SELECT key,value FROM settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := map[string][]byte{}
	for rows.Next() {
		var k string
		var v []byte
		if err = rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		items[k] = v
	}
	return items, rows.Err()
}
func (d *DB) SetSetting(ctx context.Context, key string, value []byte, uid string) error {
	_, err := d.pool.Exec(ctx, `INSERT INTO settings(key,value,updated_by) VALUES($1,$2::jsonb,$3::uuid)
	ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_by=excluded.updated_by,updated_at=now()`, key, string(value), uid)
	return err
}

func (d *DB) CreateFirstAdmin(ctx context.Context, username, hash string) (User, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(763846265)"); err != nil {
		return User{}, err
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return User{}, err
	}
	if count != 0 {
		return User{}, errors.New("bootstrap refused: users already exist")
	}
	u, err := scanUser(tx.QueryRow(ctx, `INSERT INTO users(username,password_hash,role) VALUES($1,$2,'admin') RETURNING id::text,username,role,enabled,created_at`, username, hash))
	if err != nil {
		return User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return u, nil
}
