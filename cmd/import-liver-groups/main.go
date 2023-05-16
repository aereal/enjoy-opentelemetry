package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aereal/enjoy-opentelemetry/adapters/db"
	"github.com/aereal/enjoy-opentelemetry/log"
	"github.com/aereal/enjoy-opentelemetry/observability"
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var dialect = goqu.Dialect("mysql")

type liverGroup struct {
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

type app struct {
	tracer trace.Tracer

	db *sqlx.DB
}

func newApp(tp trace.TracerProvider) *app {
	return &app{
		tracer: tp.Tracer("cmd/import-liver-groups"),
	}
}

func (a *app) run(ctx context.Context) (err error) {
	ctx, span := a.tracer.Start(ctx, "run")
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

	a.db, err = db.New(os.Getenv("DSN"))
	if err != nil {
		return fmt.Errorf("db.New: %w", err)
	}

	var (
		groups        []liverGroup
		liverIDByName map[string]int64
	)
	eg, prepareCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		groups, err = a.loadJSONData(prepareCtx)
		return err
	})
	eg.Go(func() error {
		var err error
		liverIDByName, err = a.getAllLivers(ctx)
		return err
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	groupIDByName, err := a.createGroups(ctx, groups)
	if err != nil {
		return err
	}

	if err := a.registerMembers(ctx, groups, liverIDByName, groupIDByName); err != nil {
		return err
	}

	if err := a.registerMembers(ctx, groups, liverIDByName, groupIDByName); err != nil {
		return err
	}

	return nil
}

func (a *app) registerMembers(ctx context.Context, data []liverGroup, liverIDByName map[string]int64, groupIDByName map[string]uint64) (err error) {
	ctx, span := a.tracer.Start(ctx, "registerMembers")
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

	builder := dialect.Insert("liver_group_members").
		Prepared(true).
		Cols("liver_group_id", "liver_id").
		OnConflict(goqu.DoNothing())
	for _, lg := range data {
		groupID, ok := groupIDByName[lg.Name]
		if !ok {
			continue
		}
		for _, memberName := range lg.Members {
			liverID, ok := liverIDByName[memberName]
			if !ok {
				continue
			}
			builder = builder.Vals(goqu.Vals{groupID, liverID})
		}
	}
	query, args, err := builder.ToSQL()
	if err != nil {
		return err
	}
	if _, err := a.db.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	return nil
}

func (a *app) createGroups(ctx context.Context, groups []liverGroup) (ret map[string]uint64, err error) {
	ctx, span := a.tracer.Start(ctx, "createGroups")
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

	values := make([][]any, 0, len(groups))
	for _, g := range groups {
		values = append(values, []any{g.Name})
	}
	query, args, err := dialect.Insert("liver_groups").
		Prepared(true).
		Cols("name").
		Vals(values...).
		OnConflict(goqu.DoNothing()).
		ToSQL()
	if err != nil {
		return nil, err
	}
	if _, err := a.db.ExecContext(ctx, query, args...); err != nil {
		return nil, err
	}
	var records []struct {
		Name string `db:"name"`
		ID   uint64 `db:"group_id"`
	}
	if err := a.db.SelectContext(ctx, &records, `select * from liver_groups order by group_id asc`); err != nil {
		return nil, err
	}
	ret = make(map[string]uint64, len(records))
	for _, r := range records {
		ret[r.Name] = r.ID
	}
	return ret, nil
}

func (a *app) loadJSONData(ctx context.Context) (groups []liverGroup, err error) {
	_, span := a.tracer.Start(ctx, "loadJSONData")
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

	f, err := os.Open("./db/liver_groups.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&groups); err != nil {
		return nil, err
	}
	span.SetAttributes(
		attribute.Int("group_count", len(groups)),
	)
	return groups, nil
}

func (a *app) getAllLivers(ctx context.Context) (_ map[string]int64, err error) {
	ctx, span := a.tracer.Start(ctx, "getAllLivers")
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

	var records []struct {
		ID   int64  `db:"liver_id"`
		Name string `db:"name"`
	}
	if err := a.db.SelectContext(ctx, &records, `select name, liver_id from livers order by liver_id asc`); err != nil {
		return nil, err
	}
	span.SetAttributes(
		attribute.Int("record_count", len(records)),
	)
	ret := make(map[string]int64, len(records))
	for _, r := range records {
		ret[r.Name] = r.ID
	}
	return ret, nil
}

func doMain() error {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, xray.Propagator{}))
	setupCtx := context.Background()
	tp, cleanup, err := setupTracerProvider(setupCtx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup(setupCtx)
	}()

	a := newApp(tp.TracerProvider)
	if err := a.run(context.Background()); err != nil {
		return err
	}

	return nil
}

var noop = func(context.Context) {}

const (
	deploymentEnv = "local"
	serviceName   = "import-liver-groups"
)

func setupTracerProvider(ctx context.Context) (*observability.Aggregate, func(context.Context), error) {
	opts := []observability.Option{
		observability.WithHTTPExporter(),
		observability.WithDeploymentEnvironment(deploymentEnv),
		observability.WithResourceName(serviceName),
	}
	aggr, err := observability.Setup(ctx, opts...)
	if err != nil {
		return nil, noop, fmt.Errorf("tracing.Setup: %w", err)
	}
	otel.SetTracerProvider(aggr.TracerProvider)
	cleanup := func(ctx context.Context) {
		_, logger := log.FromContext(ctx)
		if err := aggr.TracerProvider.Shutdown(ctx); err != nil {
			logger.Info("failed to cleanup otel trace provider", zap.Error(err))
		}
		if err := aggr.MetricProvider.Shutdown(ctx); err != nil {
			logger.Info("failed to cleanup otel meteric provider", zap.Error(err))
		}
	}
	return aggr, cleanup, nil
}

func main() {
	if err := doMain(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
