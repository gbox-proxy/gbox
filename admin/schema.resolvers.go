package admin

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"github.com/gbox-proxy/gbox/admin/generated"
	"go.uber.org/zap"
)

func (r *mutationResolver) PurgeAll(ctx context.Context) (bool, error) {
	if err := r.purger.PurgeQueryResultBySchema(ctx, r.upstreamSchema); err != nil {
		r.logger.Warn("fail to purge query result by operation name", zap.Error(err))

		return false, nil
	}

	return true, nil
}

func (r *mutationResolver) PurgeOperation(ctx context.Context, name string) (bool, error) {
	if err := r.purger.PurgeQueryResultByOperationName(ctx, name); err != nil {
		r.logger.Warn("fail to purge query result by operation name", zap.Error(err))

		return false, nil
	}

	return true, nil
}

func (r *mutationResolver) PurgeTypeKey(ctx context.Context, typeArg string, field string, key string) (bool, error) {
	if err := r.purger.PurgeQueryResultByTypeKey(ctx, typeArg, field, key); err != nil {
		r.logger.Warn("fail to purge query result by type key", zap.Error(err))

		return false, nil
	}

	return true, nil
}

func (r *mutationResolver) PurgeQueryRootField(ctx context.Context, field string) (bool, error) {
	if err := r.purger.PurgeQueryResultByTypeField(ctx, r.upstreamSchema.QueryTypeName(), field); err != nil {
		r.logger.Warn("fail to purge query result by root field", zap.Error(err))

		return false, nil
	}

	return true, nil
}

func (r *mutationResolver) PurgeType(ctx context.Context, typeArg string) (bool, error) {
	if err := r.purger.PurgeQueryResultByTypeName(ctx, typeArg); err != nil {
		r.logger.Warn("fail to purge query result by type", zap.Error(err))

		return false, err
	}

	return true, nil
}

func (r *queryResolver) Dummy(ctx context.Context) (string, error) {
	return "no query fields exists", nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
