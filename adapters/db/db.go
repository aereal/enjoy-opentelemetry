package db

import (
	"fmt"

	"github.com/XSAM/otelsql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/trace"
)

func New(tp trace.TracerProvider, dsn string) (*sqlx.DB, error) {
	db, err := otelsql.Open("mysql", dsn, otelsql.WithTracerProvider(tp))
	if err != nil {
		return nil, fmt.Errorf("otelsql.Open: %w", err)
	}
	return sqlx.NewDb(db, "mysql"), nil
}
