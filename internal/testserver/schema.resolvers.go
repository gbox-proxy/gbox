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
		{
			ID:   1,
			Name: "A",
			Books: []*model.BookTest{
				{
					ID:    1,
					Title: "A - Book 1",
				},
				{
					ID:    2,
					Title: "A - Book 2",
				},
			},
		},
		{
			ID:   2,
			Name: "B",
			Books: []*model.BookTest{
				{
					ID:    3,
					Title: "B - Book 1",
				},
			},
		},
		// Test ID 3 will be missing in purging tags debug header.
	}, nil
}

func (r *queryTestResolver) Users(ctx context.Context) ([]*model.UserTest, error) {
	return []*model.UserTest{
		{
			ID:   1,
			Name: "A",
			Books: []*model.BookTest{
				{
					ID:    1,
					Title: "A - Book 1",
				},
				{
					ID:    2,
					Title: "A - Book 2",
				},
			},
		},
		{
			ID:   2,
			Name: "B",
			Books: []*model.BookTest{
				{
					ID:    3,
					Title: "B - Book 1",
				},
			},
		},
		{
			ID:   3,
			Name: "C",
			Books: []*model.BookTest{
				{
					ID:    4,
					Title: "C - Book 1",
				},
			},
		},
	}, nil
}

func (r *queryTestResolver) Books(ctx context.Context) ([]*model.BookTest, error) {
	return []*model.BookTest{
		{
			ID:    1,
			Title: "A - Book 1",
		},
		{
			ID:    2,
			Title: "A - Book 2",
		},
		{
			ID:    3,
			Title: "B - Book 1",
		},
		{
			ID:    4,
			Title: "C - Book 1",
		},
	}, nil
}

// MutationTest returns generated.MutationTestResolver implementation.
func (r *Resolver) MutationTest() generated.MutationTestResolver { return &mutationTestResolver{r} }

// QueryTest returns generated.QueryTestResolver implementation.
func (r *Resolver) QueryTest() generated.QueryTestResolver { return &queryTestResolver{r} }

type (
	mutationTestResolver struct{ *Resolver }
	queryTestResolver    struct{ *Resolver }
)
