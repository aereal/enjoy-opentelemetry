package domain

import (
	"context"
	"errors"
	"strconv"

	"github.com/aereal/enjoy-opentelemetry/observability"
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrDBIsNil = errors.New("db is required")

	keyLiverName = attribute.Key("liver.name")
	dialect      = goqu.Dialect("mysql")
)

type newRepositoryConfig struct {
	tp trace.TracerProvider
	mp metric.MeterProvider
	db *sqlx.DB
}

var _ newRepositoryOptioner = (*newRepositoryConfig)(nil)

type newRepositoryOptioner interface {
	setTracerProvider(trace.TracerProvider)
	setMeterProvider(metric.MeterProvider)
	setDB(db *sqlx.DB)
}

func (c *newRepositoryConfig) setTracerProvider(tp trace.TracerProvider) {
	c.tp = tp
}

func (c *newRepositoryConfig) setMeterProvider(mp metric.MeterProvider) {
	c.mp = mp
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

func WithMetricProvider(mp metric.MeterProvider) NewRepositoryOption {
	return func(c newRepositoryOptioner) {
		c.setMeterProvider(mp)
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
	if cfg.mp == nil {
		cfg.mp = otel.GetMeterProvider()
	}
	r := &LiverGroupRepository{
		tracer: cfg.tp.Tracer("domain.LiverGroupRepository"),
		db:     cfg.db,
		meter:  cfg.mp.Meter("domain.LiverGroupRepository"),
	}
	r.tables.livers = goqu.T("livers")
	r.tables.liverGroups = goqu.T("liver_groups")
	r.tables.liverGroupMembers = goqu.T("liver_group_members")
	var err error
	if r.measurements.fetchedResultCount, err = r.meter.Int64Counter(observability.MetricNames.RepositoryFetchedResultCount); err != nil {
		return nil, err
	}
	return r, nil
}

type LiverGroupRepository struct {
	tracer trace.Tracer
	meter  metric.Meter
	db     *sqlx.DB
	tables struct {
		livers, liverGroups, liverGroupMembers exp.IdentifierExpression
	}
	measurements struct {
		fetchedResultCount metric.Int64Counter
	}
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
	query, args, err := dialect.From(r.tables.liverGroups).
		Select(r.tables.liverGroups.All(), r.tables.liverGroupMembers.Col("liver_id")).
		InnerJoin(
			r.tables.liverGroupMembers,
			goqu.On(
				goqu.And(
					r.tables.liverGroupMembers.Col("liver_group_id").Eq(r.tables.liverGroups.Col("liver_group_id")),
					r.tables.liverGroupMembers.In(liverIDs),
				),
			)).
		ToSQL()
	if err != nil {
		return nil, err
	}
	if err := r.db.SelectContext(ctx, &groups, query, args...); err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int("count", len(groups)))
	r.measurements.fetchedResultCount.Add(
		ctx,
		int64(len(groups)),
		metric.WithAttributes(observability.AttrDBTable(r.tables.liverGroups.GetTable())))
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
	query, args, err := dialect.From(r.tables.liverGroups).
		Select(r.tables.liverGroups.All()).
		InnerJoin(
			r.tables.liverGroupMembers,
			goqu.On(
				goqu.And(
					r.tables.liverGroupMembers.Col("liver_group_id").Eq(r.tables.liverGroups.Col("liver_group_id")),
					r.tables.liverGroupMembers.Eq(liverID),
				),
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
	r.measurements.fetchedResultCount.Add(
		ctx,
		int64(len(groups)),
		metric.WithAttributes(observability.AttrDBTable(r.tables.liverGroups.GetTable())))
	return groups, nil
}

type LiverRepository struct {
	tracer trace.Tracer
	meter  metric.Meter
	db     *sqlx.DB
	tables struct {
		livers exp.IdentifierExpression
	}
	measurements struct {
		fetchedResultCount metric.Int64Counter
		insertedCount      metric.Int64Counter
	}
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
	if cfg.mp == nil {
		cfg.mp = otel.GetMeterProvider()
	}
	r := &LiverRepository{
		db:     cfg.db,
		tracer: cfg.tp.Tracer("domain.LiverRepository"),
		meter:  cfg.mp.Meter("domain.LiverRepository"),
	}
	r.tables.livers = goqu.T("livers")
	var err error
	if r.measurements.fetchedResultCount, err = r.meter.Int64Counter(observability.MetricNames.RepositoryFetchedResultCount); err != nil {
		return nil, err
	}
	if r.measurements.insertedCount, err = r.meter.Int64Counter(observability.MetricNames.RepositoryInsertedCount); err != nil {
		return nil, err
	}
	return r, nil
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

	query, args, err := dialect.
		From(r.tables.livers).
		Insert().
		Cols(r.tables.livers.Col("name")).
		Vals(goqu.Vals{name}).
		ToSQL()
	if err != nil {
		return err
	}
	rows, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if affected, err := rows.RowsAffected(); err == nil {
		r.measurements.insertedCount.Add(ctx, affected)
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

	query, args, err := dialect.
		From(r.tables.livers).
		Where(r.tables.livers.Col("name").Eq(name)).
		Limit(1).
		ToSQL()
	if err != nil {
		return nil, err
	}
	var liver Liver
	if err := r.db.GetContext(ctx, &liver, query, args...); err != nil {
		return nil, err
	}
	r.measurements.fetchedResultCount.Add(ctx, 1, metric.WithAttributes(observability.AttrDBTable(r.tables.livers.GetTable())))
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
	orderColumn := r.tables.livers.Col("liver_id")
	var orderExp exp.OrderedExpression
	if cfg.direction == OrderDirectionDesc {
		orderExp = orderColumn.Desc()
	} else {
		orderExp = orderColumn.Asc()
	}
	qb := dialect.
		From(r.tables.livers).
		Limit(limit + 1).
		Order(orderExp)
	if cfg.fromLiverID != 0 {
		qb = qb.Where(r.tables.livers.Col("liver_id").Gt(cfg.fromLiverID))
	}
	query, args, err := qb.ToSQL()
	if err != nil {
		return nil, false, err
	}
	livers = make([]*Liver, 0, limit+1)
	if err := r.db.SelectContext(ctx, &livers, query, args...); err != nil {
		return nil, false, err
	}
	r.measurements.fetchedResultCount.Add(ctx, int64(len(livers)), metric.WithAttributes(observability.AttrDBTable(r.tables.livers.GetTable())))
	if len(livers) > int(limit) {
		livers = livers[:limit]
		hasNext = true
	}
	return livers, hasNext, nil
}
