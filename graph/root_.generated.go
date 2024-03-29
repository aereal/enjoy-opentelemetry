// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package graph

import (
	"bytes"
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/introspection"
	"github.com/aereal/enjoy-opentelemetry/graph/models"
	gqlparser "github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// NewExecutableSchema creates an ExecutableSchema from the ResolverRoot interface.
func NewExecutableSchema(cfg Config) graphql.ExecutableSchema {
	return &executableSchema{
		resolvers:  cfg.Resolvers,
		directives: cfg.Directives,
		complexity: cfg.Complexity,
	}
}

type Config struct {
	Resolvers  ResolverRoot
	Directives DirectiveRoot
	Complexity ComplexityRoot
}

type ResolverRoot interface {
	Liver() LiverResolver
	LiverConnection() LiverConnectionResolver
	LiverEdge() LiverEdgeResolver
	LiverGroupConnetion() LiverGroupConnetionResolver
	Mutation() MutationResolver
	Query() QueryResolver
}

type DirectiveRoot struct {
	Authenticate func(ctx context.Context, obj interface{}, next graphql.Resolver, scopes []models.Scope) (res interface{}, err error)
}

type ComplexityRoot struct {
	Group struct {
		Name func(childComplexity int) int
	}

	Liver struct {
		DebutedOn      func(childComplexity int) int
		EnrollmentDays func(childComplexity int) int
		Groups         func(childComplexity int, first *int, after *models.Cursor) int
		Name           func(childComplexity int) int
		RetiredOn      func(childComplexity int) int
		Status         func(childComplexity int) int
	}

	LiverConnection struct {
		Edges    func(childComplexity int) int
		PageInfo func(childComplexity int) int
	}

	LiverEdge struct {
		Cursor func(childComplexity int) int
		Node   func(childComplexity int) int
	}

	LiverGroupConnetion struct {
		Edges    func(childComplexity int) int
		PageInfo func(childComplexity int) int
	}

	LiverGroupEdge struct {
		Cursor func(childComplexity int) int
		Node   func(childComplexity int) int
	}

	Mutation struct {
		RegisterLiver func(childComplexity int, name string) int
	}

	PageInfo struct {
		EndCursor       func(childComplexity int) int
		HasNextPage     func(childComplexity int) int
		HasPreviousPage func(childComplexity int) int
		StartCursor     func(childComplexity int) int
	}

	Query struct {
		Liver  func(childComplexity int, name string) int
		Livers func(childComplexity int, first *int, after *models.Cursor, orderBy *models.LiverOrder) int
	}
}

type executableSchema struct {
	resolvers  ResolverRoot
	directives DirectiveRoot
	complexity ComplexityRoot
}

func (e *executableSchema) Schema() *ast.Schema {
	return parsedSchema
}

func (e *executableSchema) Complexity(typeName, field string, childComplexity int, rawArgs map[string]interface{}) (int, bool) {
	ec := executionContext{nil, e}
	_ = ec
	switch typeName + "." + field {

	case "Group.name":
		if e.complexity.Group.Name == nil {
			break
		}

		return e.complexity.Group.Name(childComplexity), true

	case "Liver.debuted_on":
		if e.complexity.Liver.DebutedOn == nil {
			break
		}

		return e.complexity.Liver.DebutedOn(childComplexity), true

	case "Liver.enrollmentDays":
		if e.complexity.Liver.EnrollmentDays == nil {
			break
		}

		return e.complexity.Liver.EnrollmentDays(childComplexity), true

	case "Liver.groups":
		if e.complexity.Liver.Groups == nil {
			break
		}

		args, err := ec.field_Liver_groups_args(context.TODO(), rawArgs)
		if err != nil {
			return 0, false
		}

		return e.complexity.Liver.Groups(childComplexity, args["first"].(*int), args["after"].(*models.Cursor)), true

	case "Liver.name":
		if e.complexity.Liver.Name == nil {
			break
		}

		return e.complexity.Liver.Name(childComplexity), true

	case "Liver.retired_on":
		if e.complexity.Liver.RetiredOn == nil {
			break
		}

		return e.complexity.Liver.RetiredOn(childComplexity), true

	case "Liver.status":
		if e.complexity.Liver.Status == nil {
			break
		}

		return e.complexity.Liver.Status(childComplexity), true

	case "LiverConnection.edges":
		if e.complexity.LiverConnection.Edges == nil {
			break
		}

		return e.complexity.LiverConnection.Edges(childComplexity), true

	case "LiverConnection.pageInfo":
		if e.complexity.LiverConnection.PageInfo == nil {
			break
		}

		return e.complexity.LiverConnection.PageInfo(childComplexity), true

	case "LiverEdge.cursor":
		if e.complexity.LiverEdge.Cursor == nil {
			break
		}

		return e.complexity.LiverEdge.Cursor(childComplexity), true

	case "LiverEdge.node":
		if e.complexity.LiverEdge.Node == nil {
			break
		}

		return e.complexity.LiverEdge.Node(childComplexity), true

	case "LiverGroupConnetion.edges":
		if e.complexity.LiverGroupConnetion.Edges == nil {
			break
		}

		return e.complexity.LiverGroupConnetion.Edges(childComplexity), true

	case "LiverGroupConnetion.pageInfo":
		if e.complexity.LiverGroupConnetion.PageInfo == nil {
			break
		}

		return e.complexity.LiverGroupConnetion.PageInfo(childComplexity), true

	case "LiverGroupEdge.cursor":
		if e.complexity.LiverGroupEdge.Cursor == nil {
			break
		}

		return e.complexity.LiverGroupEdge.Cursor(childComplexity), true

	case "LiverGroupEdge.node":
		if e.complexity.LiverGroupEdge.Node == nil {
			break
		}

		return e.complexity.LiverGroupEdge.Node(childComplexity), true

	case "Mutation.registerLiver":
		if e.complexity.Mutation.RegisterLiver == nil {
			break
		}

		args, err := ec.field_Mutation_registerLiver_args(context.TODO(), rawArgs)
		if err != nil {
			return 0, false
		}

		return e.complexity.Mutation.RegisterLiver(childComplexity, args["name"].(string)), true

	case "PageInfo.endCursor":
		if e.complexity.PageInfo.EndCursor == nil {
			break
		}

		return e.complexity.PageInfo.EndCursor(childComplexity), true

	case "PageInfo.hasNextPage":
		if e.complexity.PageInfo.HasNextPage == nil {
			break
		}

		return e.complexity.PageInfo.HasNextPage(childComplexity), true

	case "PageInfo.hasPreviousPage":
		if e.complexity.PageInfo.HasPreviousPage == nil {
			break
		}

		return e.complexity.PageInfo.HasPreviousPage(childComplexity), true

	case "PageInfo.startCursor":
		if e.complexity.PageInfo.StartCursor == nil {
			break
		}

		return e.complexity.PageInfo.StartCursor(childComplexity), true

	case "Query.liver":
		if e.complexity.Query.Liver == nil {
			break
		}

		args, err := ec.field_Query_liver_args(context.TODO(), rawArgs)
		if err != nil {
			return 0, false
		}

		return e.complexity.Query.Liver(childComplexity, args["name"].(string)), true

	case "Query.livers":
		if e.complexity.Query.Livers == nil {
			break
		}

		args, err := ec.field_Query_livers_args(context.TODO(), rawArgs)
		if err != nil {
			return 0, false
		}

		return e.complexity.Query.Livers(childComplexity, args["first"].(*int), args["after"].(*models.Cursor), args["orderBy"].(*models.LiverOrder)), true

	}
	return 0, false
}

func (e *executableSchema) Exec(ctx context.Context) graphql.ResponseHandler {
	rc := graphql.GetOperationContext(ctx)
	ec := executionContext{rc, e}
	inputUnmarshalMap := graphql.BuildUnmarshalerMap(
		ec.unmarshalInputLiverOrder,
	)
	first := true

	switch rc.Operation.Operation {
	case ast.Query:
		return func(ctx context.Context) *graphql.Response {
			if !first {
				return nil
			}
			first = false
			ctx = graphql.WithUnmarshalerMap(ctx, inputUnmarshalMap)
			data := ec._Query(ctx, rc.Operation.SelectionSet)
			var buf bytes.Buffer
			data.MarshalGQL(&buf)

			return &graphql.Response{
				Data: buf.Bytes(),
			}
		}
	case ast.Mutation:
		return func(ctx context.Context) *graphql.Response {
			if !first {
				return nil
			}
			first = false
			ctx = graphql.WithUnmarshalerMap(ctx, inputUnmarshalMap)
			data := ec._Mutation(ctx, rc.Operation.SelectionSet)
			var buf bytes.Buffer
			data.MarshalGQL(&buf)

			return &graphql.Response{
				Data: buf.Bytes(),
			}
		}

	default:
		return graphql.OneShot(graphql.ErrorResponse(ctx, "unsupported GraphQL operation"))
	}
}

type executionContext struct {
	*graphql.OperationContext
	*executableSchema
}

func (ec *executionContext) introspectSchema() (*introspection.Schema, error) {
	if ec.DisableIntrospection {
		return nil, errors.New("introspection disabled")
	}
	return introspection.WrapSchema(parsedSchema), nil
}

func (ec *executionContext) introspectType(name string) (*introspection.Type, error) {
	if ec.DisableIntrospection {
		return nil, errors.New("introspection disabled")
	}
	return introspection.WrapTypeFromDef(parsedSchema, parsedSchema.Types[name]), nil
}

var sources = []*ast.Source{
	{Name: "../schemata/main.gql", Input: `directive @authenticate(scopes: [Scope!]) on FIELD_DEFINITION

scalar Time

scalar Cursor

enum Scope {
  READ
  WRITE
}

enum LiverStatus {
  ANNOUNCED
  DEBUTED
  RETIRED
}

type Group {
  name: String!
}

type LiverGroupEdge {
  node: Group!
  cursor: Cursor!
}

type LiverGroupConnetion {
  edges: [LiverGroupEdge!]!
  pageInfo: PageInfo!
}

type Liver {
  name: String!
  debuted_on: Time!
  retired_on: Time
  status: LiverStatus!
  enrollmentDays: Int!
  groups(first: Int, after: Cursor): LiverGroupConnetion!
}

type PageInfo {
  hasPreviousPage: Boolean!
  hasNextPage: Boolean!
  startCursor: Cursor
  endCursor: Cursor
}

type LiverEdge {
  node: Liver!
  cursor: Cursor!
}

type LiverConnection {
  edges: [LiverEdge!]!
  pageInfo: PageInfo!
}

enum OrderDirection {
  ASC
  DESC
}

enum LiverOrderField {
  DATABASE_ID
}

input LiverOrder {
  field: LiverOrderField!
  direction: OrderDirection!
}

type Query {
  liver(name: String!): Liver @authenticate(scopes: [READ])
  livers(
    first: Int = 0,
    after: Cursor,
    orderBy: LiverOrder
  ): LiverConnection! @authenticate(scopes: [READ])
}

type Mutation {
  registerLiver(name: String!): Boolean! @authenticate(scopes: [WRITE])
}
`, BuiltIn: false},
}
var parsedSchema = gqlparser.MustLoadSchema(sources...)
