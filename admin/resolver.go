package admin

//go:generate go run -mod=mod github.com/99designs/gqlgen generate

import (
	"context"
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"go.uber.org/zap"
)

type QueryResultCachePurger interface {
	PurgeQueryResultByOperationName(context.Context, string) error
	PurgeQueryResultByTypeName(context.Context, string) error
	PurgeQueryResultByTypeField(ctx context.Context, typeName, fieldName string) error
	PurgeQueryResultByTypeKey(ctx context.Context, typeName, fieldName string, value interface{}) error
}

type Resolver struct {
	upstreamSchema           *graphql.Schema
	upstreamSchemaDefinition *ast.Document
	purger                   QueryResultCachePurger
	logger                   *zap.Logger
}

func NewResolver(s *graphql.Schema, d *ast.Document, l *zap.Logger, p QueryResultCachePurger) *Resolver {
	return &Resolver{
		upstreamSchema:           s,
		upstreamSchemaDefinition: d,
		logger:                   l,
		purger:                   p,
	}
}
