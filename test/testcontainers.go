package main

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"
)

func (m *Main) Testcontainers(ctx context.Context) error {
	examples := dag.Git("https://github.com/testcontainers/testcontainers-go").
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
		eg.Go(func() error {
			_, err := dag.Golang().
				WithVersion("1").
				WithSource(examples).
				Container().
				Pipeline(suite).
				With(dag.Testcontainers().Setup).
				WithEnvVariable("BUST", time.Now().String()).
				WithWorkdir("/src/examples").
				WithWorkdir(suite).
				WithFocus().
				WithExec([]string{"test", "-v", "."}).
				Sync(ctx)
			return err
		})
	}

	return eg.Wait()
}
