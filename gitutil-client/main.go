package main

import (
	"context"
)

type GitutilClient struct{}

func (m *GitutilClient) Latest(ctx context.Context) (string, error) {
	return dag.Gitutil().LatestSemverTag(ctx,
		dag.Container().From("alpine/git"),
		"https://github.com/vito/booklit",
		"")
}
