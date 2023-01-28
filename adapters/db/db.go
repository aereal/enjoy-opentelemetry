package db

import (
	"fmt"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var defaultLoc *time.Location

func init() {
	var err error
	defaultLoc, err = time.LoadLocation("Asia/Tokyo")
	if err != nil {
		panic(err)
	}
}

type config struct {
	tp trace.TracerProvider
}

type Option func(*config)

func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *config) {
		c.tp = tp
	}
}

func New(dsn string, opts ...Option) (*sqlx.DB, error) {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.tp == nil {
		cfg.tp = otel.GetTracerProvider()
	}

	dbCfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	dbCfg.ParseTime = true
	dbCfg.Loc = defaultLoc
	db, err := otelsql.Open("mysql", dbCfg.FormatDSN(), otelsql.WithTracerProvider(cfg.tp), otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}))
	if err != nil {
		return nil, fmt.Errorf("otelsql.Open: %w", err)
	}
	return sqlx.NewDb(db, "mysql"), nil
}
