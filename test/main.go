package main

import (
	"context"
)

type Main struct{}

func (m *Main) Go(ctx context.Context) *Container {
	return dag.Apko().Wolfi([]string{"go"})
}
