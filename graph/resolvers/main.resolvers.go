package resolvers

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.31

import (
	"context"
	"encoding/json"

	"github.com/aereal/enjoy-opentelemetry/domain"
	"github.com/aereal/enjoy-opentelemetry/graph"
	"github.com/aereal/enjoy-opentelemetry/graph/loaders"
	"github.com/aereal/enjoy-opentelemetry/graph/models"
)

// Groups is the resolver for the groups field.
func (r *liverResolver) Groups(ctx context.Context, obj *domain.Liver, first *int, after *models.Cursor) (*models.LiverGroupConnection, error) {
	var f int
	if first != nil {
		f = *first
	}
	cv := &models.GroupCursorValue{}
	if after != nil {
		if err := json.Unmarshal(after.Value, cv); err != nil {
			return nil, err
		}
	}

	groups, err := loaders.GetBelongingGroupsByLiverID(ctx, obj.ID)
	if err != nil {
		return nil, err
	}

	edges := make([]*models.LiverGroupEdge, 0, len(groups))
	for _, group := range groups {
		if group.ID < cv.GroupID {
			continue
		}
		edge := &models.LiverGroupEdge{Node: group}
		edges = append(edges, edge)
		if len(edges) >= f {
			break
		}
	}
	return &models.LiverGroupConnection{
		Edges:   edges,
		HasNext: len(groups) > f,
	}, nil
}

// PageInfo is the resolver for the pageInfo field.
func (r *liverConnectionResolver) PageInfo(ctx context.Context, obj *models.LiverConnection) (*models.PageInfo, error) {
	if len(obj.Edges) == 0 {
		return &models.PageInfo{}, nil
	}
	pi, err := models.NewPageInfo(obj.Edges, obj.HasNext)
	if err != nil {
		return nil, err
	}
	return pi, nil
}

// Node is the resolver for the node field.
func (r *liverEdgeResolver) Node(ctx context.Context, obj *models.LiverEdge) (*domain.Liver, error) {
	return obj.Liver, nil
}

// PageInfo is the resolver for the pageInfo field.
func (r *liverGroupConnetionResolver) PageInfo(ctx context.Context, obj *models.LiverGroupConnection) (*models.PageInfo, error) {
	return models.NewPageInfo(obj.Edges, obj.HasNext)
}

// RegisterLiver is the resolver for the registerLiver field.
func (r *mutationResolver) RegisterLiver(ctx context.Context, name string) (bool, error) {
	if err := r.liverRepository.RegisterLiver(ctx, name); err != nil {
		return false, err
	}
	return true, nil
}

// Liver is the resolver for the liver field.
func (r *queryResolver) Liver(ctx context.Context, name string) (*domain.Liver, error) {
	liver, err := r.liverRepository.GetLiverByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return liver, nil
}

// Livers is the resolver for the livers field.
func (r *queryResolver) Livers(ctx context.Context, first *int, after *models.Cursor, orderBy *models.LiverOrder) (*models.LiverConnection, error) {
	if first == nil || *first <= 0 {
		return &models.LiverConnection{}, nil
	}
	firstInt := *first
	var direction domain.OrderDirection
	if orderBy == nil {
		direction = domain.OrderDirectionAsc
	}
	cv := &models.LiverCursorValue{}
	if after != nil {
		if err := json.Unmarshal(after.Value, cv); err != nil {
			return nil, err
		}
	}
	livers, hasNext, err := r.liverRepository.GetLivers(ctx, uint(firstInt), domain.WithOrderDirection(direction))
	if err != nil {
		return nil, err
	}
	edges := make([]*models.LiverEdge, len(livers))
	for i, liver := range livers {
		edges[i] = &models.LiverEdge{Liver: liver}
	}
	conn := &models.LiverConnection{
		Edges:   edges,
		HasNext: hasNext,
	}
	return conn, nil
}

// Liver returns graph.LiverResolver implementation.
func (r *Resolver) Liver() graph.LiverResolver { return &liverResolver{r} }

// LiverConnection returns graph.LiverConnectionResolver implementation.
func (r *Resolver) LiverConnection() graph.LiverConnectionResolver {
	return &liverConnectionResolver{r}
}

// LiverEdge returns graph.LiverEdgeResolver implementation.
func (r *Resolver) LiverEdge() graph.LiverEdgeResolver { return &liverEdgeResolver{r} }

// LiverGroupConnetion returns graph.LiverGroupConnetionResolver implementation.
func (r *Resolver) LiverGroupConnetion() graph.LiverGroupConnetionResolver {
	return &liverGroupConnetionResolver{r}
}

// Mutation returns graph.MutationResolver implementation.
func (r *Resolver) Mutation() graph.MutationResolver { return &mutationResolver{r} }

// Query returns graph.QueryResolver implementation.
func (r *Resolver) Query() graph.QueryResolver { return &queryResolver{r} }

type liverResolver struct{ *Resolver }
type liverConnectionResolver struct{ *Resolver }
type liverEdgeResolver struct{ *Resolver }
type liverGroupConnetionResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
