package main

import (
	"context"
	"path"

	"golang.org/x/sync/errgroup"

	"dagger/test/internal/dagger"
	"dagger/test/internal/telemetry"
)

func (m *Main) Testcontainers(ctx context.Context) error {
	repo := dag.Git("https://github.com/testcontainers/testcontainers-go").
		Commit("504645849200304ea4257efee027e70276cf11c9").
		Tree()

	// Optional: start a Docker daemon that'll be kept around across all suites
	// even if there is idle time due to load
	_, err := dag.Testcontainers().DockerService().Start(ctx)
	if err != nil {
		return err
	}

	eg := new(errgroup.Group)
	for _, suite := range []string{
		"cockroachdb",
		"consul",
		"nginx",
		"toxiproxy",
	} {
		suite := suite
		eg.Go(func() (rerr error) {
			ctx, span := Tracer().Start(ctx, suite)
			defer telemetry.End(span, func() error { return rerr })
			_, err := dag.
				Go(dagger.GoOpts{
					Base: dag.Go().Base().With(dag.Testcontainers().Setup),
				}).
				Test(repo, dagger.GoTestOpts{
					Subdir:  path.Join("examples", suite),
					Verbose: true,
				}).
				Sync(ctx)
			return err
		})
	}

	return eg.Wait()
}
