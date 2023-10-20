package main

import (
	"context"
	"fmt"
	"strconv"
)

type Concourse struct{}

type QuickstartOpts struct {
	// Image   string `doc:"concourse image tag" default:"concourse/concourse:7.10@sha256:e45dffda72e32e11e5790530f8b41a23af4a49a21d585967c4f00c3cf3b12164"`
	// containerd is not working on macOS when using the official image
	Image   string `doc:"concourse image tag" default:"rdclda/concourse:7.9.1@sha256:963929885c7a9b917dfd98244757b4f32f64b3c322c842a2da20aca6e32787b2"`
	DBImage string `doc:"concourse db image tag" default:"postgres:15.4-alpine@sha256:6f5520d31e1223facb11066b6c99333ffabf190a5d48c50d615b858602f5f8b5"`
	WebPort int    `doc:"concourse web port" default:"8080"`
}

func (m *Concourse) Quickstart(ctx context.Context, opts QuickstartOpts) *Service {
	workerWorkDir := dag.CacheVolume("concourse-worker-work-dir")

	return dag.Container().From(opts.Image).
		WithMountedCache("/concourse-worker-work-dir", workerWorkDir).
		WithEnvVariable("CONCOURSE_WORKER_WORK_DIR", "/concourse-worker-work-dir").
		WithServiceBinding("db", m.postgresql(ctx, opts.DBImage)).
		WithExposedPort(opts.WebPort).
		WithEnvVariable("CONCOURSE_BIND_PORT", strconv.Itoa(opts.WebPort)).
		WithEnvVariable("CONCOURSE_POSTGRES_HOST", "db").
		WithEnvVariable("CONCOURSE_POSTGRES_DATABASE", "concourse").
		WithEnvVariable("CONCOURSE_POSTGRES_USER", "concourse").
		WithEnvVariable("CONCOURSE_POSTGRES_PASSWORD", "concourse").
		WithEnvVariable("CONCOURSE_ADD_LOCAL_USER", "dagger:dagger").
		WithEnvVariable("CONCOURSE_MAIN_TEAM_LOCAL_USER", "dagger").
		WithEnvVariable("CONCOURSE_CLUSTER_NAME", "dagger").
		WithEnvVariable("CONCOURSE_WORKER_RUNTIME", "containerd").
		// containerd is not working on macOS when using the official image
		// WithEnvVariable("CONCOURSE_WORKER_RUNTIME", "houdini").
		WithEnvVariable("CONCOURSE_WORKER_BAGGAGECLAIM_DRIVER", "overlay").
		WithEnvVariable("CONCOURSE_ENABLE_PIPELINE_INSTANCES", "true").
		WithEnvVariable("CONCOURSE_ENABLE_ACROSS_STEP", "true").
		WithEnvVariable("CONCOURSE_EXTERNAL_URL", fmt.Sprintf("https://localhost:%d", opts.WebPort)).
		WithEntrypoint(nil).
		WithExec([]string{"/usr/local/bin/entrypoint.sh", "quickstart"}, ContainerWithExecOpts{
			InsecureRootCapabilities: true,
		}).
		AsService()
}

func (m *Concourse) postgresql(ctx context.Context, image string) *Service {
	return dag.Container().From(image).
		WithExposedPort(5432).
		WithEnvVariable("POSTGRES_DB", "concourse").
		WithEnvVariable("POSTGRES_USER", "concourse").
		WithEnvVariable("POSTGRES_PASSWORD", "concourse").
		WithEnvVariable("PGDATA", "/database").
		WithMountedCache("/database", dag.CacheVolume("concourse-db")).
		AsService()
}
