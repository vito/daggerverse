package main

import (
	"context"

	"dagger/test/internal/dagger"
)

func (m *Main) Alpine(
	ctx context.Context,
	packages []string,
	// +optional
	// +default="edge"
	branch string) *dagger.Container {
	return dag.Apko().Alpine(packages, dagger.ApkoAlpineOpts{
		Branch: branch,
	})
}

func (m *Main) Wolfi(ctx context.Context, packages []string) *dagger.Container {
	return dag.Apko().Wolfi(packages)
}
