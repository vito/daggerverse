package main

import (
	"context"
	"fmt"
	"strconv"
)

type Concourse struct{}

type CreateOpts struct {
	ImageTag   string `doc:"concourse image tag" default:"concourse/concourse:7.10@sha256:e45dffda72e32e11e5790530f8b41a23af4a49a21d585967c4f00c3cf3b12164"`
	DbImageTag string `doc:"concourse db image tag" default:"postgres:15.4-alpine@sha256:6f5520d31e1223facb11066b6c99333ffabf190a5d48c50d615b858602f5f8b5"`
	webPort    int    `doc:"concourse web port" default:"8080"`
}

func (m *Concourse) Quickstart(ctx context.Context, opts CreateOpts) *Service {
	workerWorkDir := dag.CacheVolume("concourse-worker-work-dir")

	return dag.Container().From(opts.ImageTag).
		WithMountedCache("/concourse-worker-work-dir", workerWorkDir).
		WithEnvVariable("CONCOURSE_WORKER_WORK_DIR", "/concourse-worker-work-dir").
		WithServiceBinding("db", m.postgresql(ctx, opts)).
		WithExposedPort(opts.webPort).
		WithEnvVariable("CONCOURSE_BIND_PORT", strconv.Itoa(opts.webPort)).
		WithEnvVariable("CONCOURSE_POSTGRES_HOST", "db").
		WithEnvVariable("CONCOURSE_POSTGRES_DATABASE", "modules").
		WithEnvVariable("CONCOURSE_POSTGRES_USER", "modules").
		WithEnvVariable("CONCOURSE_POSTGRES_PASSWORD", "modules").
		WithEnvVariable("CONCOURSE_ADD_LOCAL_USER", "modules:modules").
		WithEnvVariable("CONCOURSE_MAIN_TEAM_LOCAL_USER", "modules").
		WithEnvVariable("CONCOURSE_CLUSTER_NAME", "modules").
		WithEnvVariable("CONCOURSE_WORKER_RUNTIME", "containerd").
		WithEnvVariable("CONCOURSE_BAGGAGECLAIM_DRIVER", "overlay").
		WithEnvVariable("CONCOURSE_ENABLE_PIPELINE_INSTANCES", "true").
		WithEnvVariable("CONCOURSE_ENABLE_ACROSS_STEP", "true").
		WithEnvVariable("CONCOURSE_EXTERNAL_URL", fmt.Sprintf("https://localhost:%s", strconv.Itoa(opts.webPort))).
		WithEntrypoint(nil).
		WithExec([]string{"/usr/local/bin/entrypoint.sh", "quickstart"}, ContainerWithExecOpts{
			InsecureRootCapabilities: true,
		}).
		Service()
}

func (m *Concourse) postgresql(ctx context.Context, opts CreateOpts) *Service {
	return dag.Container().From(opts.DbImageTag).
		WithExposedPort(5432).
		WithEnvVariable("POSTGRES_DB", "modules").
		WithEnvVariable("POSTGRES_USER", "modules").
		WithEnvVariable("POSTGRES_PASSWORD", "modules").
		WithEnvVariable("PGDATA", "/database").
		WithMountedCache("/database", dag.CacheVolume("concourse-db")).
		Service()
}
