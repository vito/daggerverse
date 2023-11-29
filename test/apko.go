package main

import "context"

func (m *Main) Alpine(ctx context.Context, packages []string, branch Optional[string]) *Container {
	return dag.Apko().Alpine(packages, branch.GetOr("edge"))
}

func (m *Main) Wolfi(ctx context.Context, packages []string) *Container {
	return dag.Apko().Wolfi(packages)
}
