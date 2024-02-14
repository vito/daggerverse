package main

import (
	"context"

	"github.com/Khan/genqlient/graphql"
)

type Boomer struct {
}

// Boom keeps calling itself with a larger and larger n until your computer
// explodes.
func (m *Boomer) Boom(
	ctx context.Context,
	// +optional
	// +default=1
	n int,
) error {
	return dag.c.MakeRequest(ctx, &graphql.Request{
		Query: "query Dig($x: Int!){boomer{boom(n: $x)}}",
		Variables: map[string]interface{}{
			"x": n + 1,
		},
	}, &graphql.Response{})
}
