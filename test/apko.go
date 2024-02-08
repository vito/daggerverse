package main

import "context"

func (m *Main) Alpine(
	ctx context.Context,
	packages []string,
	// +optional
	// +default=edge
	branch string) *Container {
	return dag.Apko().Alpine(packages, branch)
}

func (m *Main) Wolfi(ctx context.Context, packages []string) *Container {
	return dag.Apko().Wolfi(packages)
}
