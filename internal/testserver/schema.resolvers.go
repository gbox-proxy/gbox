package testserver

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/gbox-proxy/gbox/internal/testserver/generated"
	"github.com/gbox-proxy/gbox/internal/testserver/model"
)

func (r *mutationTestResolver) UpdateUsers(ctx context.Context) ([]*model.UserTest, error) {
	return []*model.UserTest{
		&model.UserTest{
			ID:   1,
			Name: "A",
		},
		&model.UserTest{
			ID:   2,
			Name: "B",
		},
		// Test ID 3 will be missing in purging tags debug header.
	}, nil
}

func (r *queryTestResolver) Users(ctx context.Context) ([]*model.UserTest, error) {
	return []*model.UserTest{
		&model.UserTest{
			ID:   1,
			Name: "A",
		},
		&model.UserTest{
			ID:   2,
			Name: "B",
		},
		&model.UserTest{
			ID:   3,
			Name: "C",
		},
	}, nil
}

// MutationTest returns generated.MutationTestResolver implementation.
func (r *Resolver) MutationTest() generated.MutationTestResolver { return &mutationTestResolver{r} }

// QueryTest returns generated.QueryTestResolver implementation.
func (r *Resolver) QueryTest() generated.QueryTestResolver { return &queryTestResolver{r} }

type mutationTestResolver struct{ *Resolver }
type queryTestResolver struct{ *Resolver }
