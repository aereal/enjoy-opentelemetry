package domain

import (
	"context"
	"errors"
	"strconv"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var dialect = goqu.Dialect("mysql")

type newLiverGroupRepositoryConfig struct {
	tp trace.TracerProvider
	db *sqlx.DB
}

var _ newLiverGroupRepositoryOptioner = (*newLiverGroupRepositoryConfig)(nil)

func (c *newLiverGroupRepositoryConfig) setTracerProvider(tp trace.TracerProvider) {
	c.tp = tp
}

func (c *newLiverGroupRepositoryConfig) setDB(db *sqlx.DB) {
	c.db = db
}

type newLiverGroupRepositoryOptioner interface {
	setTracerProvider(trace.TracerProvider)
	setDB(db *sqlx.DB)
}

func WithDB(db *sqlx.DB) NewLiverGroupRepositoryOption {
	return func(c newLiverGroupRepositoryOptioner) {
		c.setDB(db)
	}
}

func WithTracerProvider(tp trace.TracerProvider) NewLiverGroupRepositoryOption {
	return func(c newLiverGroupRepositoryOptioner) {
		c.setTracerProvider(tp)
	}
}

type NewLiverGroupRepositoryOption func(c newLiverGroupRepositoryOptioner)

func NewLiverGroupRepository(opts ...NewLiverGroupRepositoryOption) (*LiverGroupRepository, error) {
	cfg := &newLiverGroupRepositoryConfig{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.tp == nil {
		cfg.tp = otel.GetTracerProvider()
	}
	if cfg.db == nil {
		return nil, errors.New("db is required")
	}
	return &LiverGroupRepository{
		tracer: cfg.tp.Tracer("domain.LiverGroupRepository"),
		db:     cfg.db,
	}, nil
}

type LiverGroupRepository struct {
	tracer trace.Tracer
	db     *sqlx.DB
}

func (r *LiverGroupRepository) GetBelongingGruopsByLiver(ctx context.Context, liverID uint64, first int) (_ []*Group, err error) {
	ctx, span := r.tracer.Start(ctx, "LiverGroupRepository.GetBelongingGruopsByLiver")
	defer func() {
		var code codes.Code
		var desc string
		if err != nil {
			desc = err.Error()
			code = codes.Error
		} else {
			code = codes.Ok
		}
		span.SetStatus(code, desc)
		span.End()
	}()
	span.SetAttributes(
		attribute.String("liver_id", strconv.FormatUint(liverID, 10)),
		attribute.Int("first", first),
	)

	var groups []*Group
	query, args, err := dialect.From("liver_groups").
		Select("liver_groups.*").
		InnerJoin(
			goqu.T("liver_group_members"),
			goqu.On(
				goqu.Ex{
					"liver_group_members.liver_group_id": goqu.I("liver_groups.liver_group_id"),
					"liver_group_members.liver_id":       liverID,
				},
			)).
		Limit(uint(first)).
		ToSQL()
	if err != nil {
		return nil, err
	}
	if err := r.db.SelectContext(ctx, &groups, query, args...); err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int("count", len(groups)))
	return groups, nil
}
