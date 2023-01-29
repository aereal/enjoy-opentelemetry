package loaders

import (
	"context"
	"errors"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/aereal/enjoy-opentelemetry/domain"
	"github.com/graph-gophers/dataloader/v7"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Aggregate struct {
	LiverGroup *dataloader.Loader[uint64, []*domain.Group]
}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationInterceptor
} = (*Aggregate)(nil)

type config struct {
	tp trace.TracerProvider
}

type Option func(c *config)

func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *config) {
		c.tp = tp
	}
}

var (
	ErrLiverGroupRepositoryRequired = errors.New("liverGroupRepository is nil")
	ErrLoaderAggregateRequired      = errors.New("loaders.Aggregate is nil")
)

func NewAggregate(liverGroupRepository *domain.LiverGroupRepository, opts ...Option) (*Aggregate, error) {
	if liverGroupRepository == nil {
		return nil, ErrLiverGroupRepositoryRequired
	}
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.tp == nil {
		cfg.tp = otel.GetTracerProvider()
	}
	liverGroupLoader := &LiverGroupLoader{
		tracer:               cfg.tp.Tracer("graph/loaders.LiverGroupLoader"),
		liverGroupRepository: liverGroupRepository,
	}
	liverGroupCache := &dataloader.NoCache[uint64, []*domain.Group]{}
	return &Aggregate{
		LiverGroup: dataloader.NewBatchedLoader(liverGroupLoader.LoadLiverGroups, dataloader.WithCache[uint64, []*domain.Group](liverGroupCache)),
	}, nil
}

func (*Aggregate) ExtensionName() string {
	return "LoaderAggregate"
}

func (*Aggregate) Validate(_ graphql.ExecutableSchema) error {
	return nil
}

type ctxKey string

var key = ctxKey("github.com/aereal/enjoy-opentelemetry/graph/loaders.LoaderAggregate")

func (a *Aggregate) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	return next(context.WithValue(ctx, key, a))
}

type LiverGroupLoader struct {
	tracer               trace.Tracer
	liverGroupRepository *domain.LiverGroupRepository
}

type GroupResult = dataloader.Result[[]*domain.Group]

func (l *LiverGroupLoader) LoadLiverGroups(ctx context.Context, keys []uint64) []*GroupResult {
	ctx, span := l.tracer.Start(ctx, "LiverGroupLoader.LoadLiverGroups")
	defer func() {
		span.End()
	}()

	belongingGroups, err := l.liverGroupRepository.GetBelongingGroupsByLivers(ctx, keys)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil
	}
	groupsByLiverID := map[uint64][]*domain.Group{}
	for _, group := range belongingGroups {
		groupsByLiverID[group.LiverID] = append(groupsByLiverID[group.LiverID], &group.Group)
	}

	results := make([]*GroupResult, len(keys))
	for i, liverID := range keys {
		if groups, ok := groupsByLiverID[liverID]; ok {
			results[i] = &GroupResult{Data: groups}
		} else {
			results[i] = &GroupResult{Error: fmt.Errorf("belonging groups not found for liver ID: %v", liverID)}
		}
	}
	return results
}

func For(ctx context.Context) *Aggregate {
	aggr, ok := ctx.Value(key).(*Aggregate)
	if !ok {
		return nil
	}
	return aggr
}

func GetBelongingGroupsByLiverID(ctx context.Context, liverID uint64) ([]*domain.Group, error) {
	loaders := For(ctx)
	if loaders == nil {
		return nil, ErrLoaderAggregateRequired
	}
	thunk := loaders.LiverGroup.Load(ctx, liverID)
	got, err := thunk()
	if err != nil {
		return nil, err
	}
	return got, nil
}
