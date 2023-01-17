package db

import (
	"fmt"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
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

func New(tp trace.TracerProvider, dsn string) (*sqlx.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ParseTime = true
	cfg.Loc = defaultLoc
	db, err := otelsql.Open("mysql", cfg.FormatDSN(), otelsql.WithTracerProvider(tp), otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}))
	if err != nil {
		return nil, fmt.Errorf("otelsql.Open: %w", err)
	}
	return sqlx.NewDb(db, "mysql"), nil
}
