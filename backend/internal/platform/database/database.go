// Package database owns the PostgreSQL connection lifecycle.
package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/StephenQiu30/then/backend/internal/platform/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	ErrUnavailable = errors.New("database unavailable")
	ErrVersion     = errors.New("database requires PostgreSQL major 18")
)

type Pool struct{ sql *sql.DB }

func Open(ctx context.Context, cfg config.Config) (*Pool, error) {
	orm, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{Logger: logger.Discard, DisableAutomaticPing: true})
	if err != nil {
		return nil, ErrUnavailable
	}
	db, err := orm.DB()
	if err != nil {
		return nil, ErrUnavailable
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, ErrUnavailable
	}
	type versionRow struct{ Version int }
	rows, err := gorm.G[versionRow](orm).Raw("SELECT current_setting('server_version_num')::int AS version").Find(ctx)
	if err != nil || len(rows) != 1 {
		_ = db.Close()
		return nil, ErrUnavailable
	}
	if rows[0].Version/10000 != 18 {
		_ = db.Close()
		return nil, ErrVersion
	}
	return &Pool{sql: db}, nil
}

func (p *Pool) Probe(ctx context.Context) error {
	if err := p.sql.PingContext(ctx); err != nil {
		return ErrUnavailable
	}
	return nil
}

func (p *Pool) Close() error { return p.sql.Close() }
