package domain

import (
	"context"
	"errors"
	"strconv"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrDBIsNil = errors.New("db is required")

	keyLiverName = attribute.Key("liver.name")
	dialect      = goqu.Dialect("mysql")
	liversTable  = dialect.From("livers")
)

type newRepositoryConfig struct {
	tp trace.TracerProvider
	db *sqlx.DB
}

var _ newRepositoryOptioner = (*newRepositoryConfig)(nil)

type newRepositoryOptioner interface {
	setTracerProvider(trace.TracerProvider)
	setDB(db *sqlx.DB)
}

func (c *newRepositoryConfig) setTracerProvider(tp trace.TracerProvider) {
	c.tp = tp
}

func (c *newRepositoryConfig) setDB(db *sqlx.DB) {
	c.db = db
}

type NewRepositoryOption func(c newRepositoryOptioner)

func WithDB(db *sqlx.DB) NewRepositoryOption {
	return func(c newRepositoryOptioner) {
		c.setDB(db)
	}
}

func WithTracerProvider(tp trace.TracerProvider) NewRepositoryOption {
	return func(c newRepositoryOptioner) {
		c.setTracerProvider(tp)
	}
}

func NewLiverGroupRepository(opts ...NewRepositoryOption) (*LiverGroupRepository, error) {
	cfg := &newRepositoryConfig{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.tp == nil {
		cfg.tp = otel.GetTracerProvider()
	}
	if cfg.db == nil {
		return nil, ErrDBIsNil
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

func (r *LiverGroupRepository) GetBelongingGroupsByLivers(ctx context.Context, liverIDs []uint64) (_ []*LiverBelongingGroup, err error) {
	ctx, span := r.tracer.Start(ctx, "LiverGroupRepository.GetBelongingGruopsByLivers")
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
	ids := make([]string, len(liverIDs))
	for i, lid := range liverIDs {
		ids[i] = strconv.FormatUint(lid, 10)
	}
	span.SetAttributes(
		attribute.StringSlice("liver_ids", ids),
	)

	var groups []*LiverBelongingGroup
	query, args, err := dialect.From("liver_groups").
		Select("liver_groups.*", "liver_group_members.liver_id").
		InnerJoin(
			goqu.T("liver_group_members"),
			goqu.On(
				goqu.Ex{
					"liver_group_members.liver_group_id": goqu.I("liver_groups.liver_group_id"),
					"liver_group_members.liver_id":       liverIDs,
				},
			)).
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

type LiverRepository struct {
	tracer trace.Tracer
	db     *sqlx.DB
}

func NewLiverRepository(opts ...NewRepositoryOption) (*LiverRepository, error) {
	var cfg newRepositoryConfig
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.db == nil {
		return nil, ErrDBIsNil
	}
	if cfg.tp == nil {
		cfg.tp = otel.GetTracerProvider()
	}
	return &LiverRepository{
		db:     cfg.db,
		tracer: cfg.tp.Tracer("domain.LiverRepository"),
	}, nil
}

func (r *LiverRepository) RegisterLiver(ctx context.Context, name string) (err error) {
	ctx, span := r.tracer.Start(ctx, "LiverRepository.RegisterLiver", trace.WithAttributes(keyLiverName.String(name)))
	defer func() {
		var code codes.Code
		var desc string
		if err != nil {
			code = codes.Error
			desc = err.Error()
			span.RecordError(err)
		} else {
			code = codes.Ok
		}
		span.SetStatus(code, desc)
		span.End()
	}()

	query, args, err := liversTable.
		Insert().
		Cols("name").
		Vals(goqu.Vals{name}).
		ToSQL()
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	return nil
}

func (r *LiverRepository) GetLiverByName(ctx context.Context, name string) (_ *Liver, err error) {
	ctx, span := r.tracer.Start(ctx, "LiverRepository.GetLiverByName", trace.WithAttributes(keyLiverName.String(name)))
	defer func() {
		var code codes.Code
		var desc string
		if err != nil {
			code = codes.Error
			desc = err.Error()
			span.RecordError(err)
		} else {
			code = codes.Ok
		}
		span.SetStatus(code, desc)
		span.End()
	}()

	query, args, err := liversTable.
		Where(goqu.C("name").Eq(name)).
		Limit(1).
		ToSQL()
	if err != nil {
		return nil, err
	}
	var liver Liver
	if err := r.db.GetContext(ctx, &liver, query, args...); err != nil {
		return nil, err
	}
	return &liver, nil
}

type getLiversConfig struct {
	fromLiverID uint64
	direction   OrderDirection
}

type GetLiversOption func(c *getLiversConfig)

func WithStartLiverID(liverID uint64) GetLiversOption {
	return func(c *getLiversConfig) {
		c.fromLiverID = liverID
	}
}

func WithOrderDirection(direction OrderDirection) GetLiversOption {
	return func(c *getLiversConfig) {
		c.direction = direction
	}
}

func (r *LiverRepository) GetLivers(ctx context.Context, limit uint, opts ...GetLiversOption) (livers []*Liver, hasNext bool, err error) {
	ctx, span := r.tracer.Start(ctx, "LiverRepository.GetLivers")
	defer func() {
		var code codes.Code
		var desc string
		if err != nil {
			code = codes.Error
			desc = err.Error()
			span.RecordError(err)
		} else {
			code = codes.Ok
		}
		span.SetStatus(code, desc)
		span.End()
	}()

	var cfg getLiversConfig
	for _, o := range opts {
		o(&cfg)
	}
	orderColumn := goqu.C("liver_id")
	var orderExp exp.OrderedExpression
	if cfg.direction == OrderDirectionDesc {
		orderExp = orderColumn.Desc()
	} else {
		orderExp = orderColumn.Asc()
	}
	qb := liversTable.
		Limit(limit + 1).
		Order(orderExp)
	if cfg.fromLiverID != 0 {
		qb = qb.Where(goqu.C("liver_id").Gt(cfg.fromLiverID))
	}
	query, args, err := qb.ToSQL()
	if err != nil {
		return nil, false, err
	}
	livers = make([]*Liver, 0, limit+1)
	if err := r.db.SelectContext(ctx, &livers, query, args...); err != nil {
		return nil, false, err
	}
	if len(livers) > int(limit) {
		livers = livers[:limit]
		hasNext = true
	}
	return livers, hasNext, nil
}
