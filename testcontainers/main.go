package main

import (
	"context"
)

// Testcontainers provides a simple interface for wrapping an existing test
// suite that uses Testcontainers.
type Testcontainers struct {
	Docker *Service
}

func defaultDind() *Service {
	return dag.Container().From("docker:dind").
		WithFocus().
		WithEnvVariable("TINI_SUBREAPER", "").
		WithExec([]string{
			"dockerd",
			"--tls=false",
			"--host=tcp://0.0.0.0:2375",
		}, ContainerWithExecOpts{
			InsecureRootCapabilities: true,
		}).
		WithExposedPort(2375).
		AsService()
}

// WithDocker allows you to override the Docker daemon used by Testcontainers.
func (m *Testcontainers) WithDocker(docker *Service) *Testcontainers {
	m.Docker = docker
	return m
}

// StartDocker allows you to start the Docker daemon in the background to
// guarantee that it stays running between test suites.
func (m *Testcontainers) StartDocker(ctx context.Context) error {
	_, err := m.docker().Start(ctx)
	return err
}

// Setup attaches a Docker daemon to the container and points Testcontainers to
// it.
func (m *Testcontainers) Setup(ctr *Container) *Container {
	return ctr.
		WithServiceBinding("docker", m.docker()).
		WithEnvVariable("DOCKER_HOST", "tcp://docker:2375").
		WithEnvVariable("TESTCONTAINERS_RYUK_DISABLED", "true")
}

func (m *Testcontainers) docker() *Service {
	if m.Docker != nil {
		return m.Docker
	} else {
		return defaultDind()
	}
}
