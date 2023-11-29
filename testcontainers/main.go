package main

// Testcontainers provides a simple interface for wrapping an existing test
// suite that uses Testcontainers.
type Testcontainers struct {
	Docker *Service
}

// WithDocker allows you to override the Docker daemon used by Testcontainers.
func (m *Testcontainers) WithDocker(docker *Service) *Testcontainers {
	m.Docker = docker
	return m
}

// DockerService exposes the Docker service so that you can start it before
// running a bunch of test suites, keeping it around across the full run even
// if there is excessive idle time due to load.
func (m *Testcontainers) DockerService() *Service {
	if m.Docker != nil {
		return m.Docker
	} else {
		return dag.Docker().Daemon().Service()
	}
}

// Setup attaches a Docker daemon to the container and points Testcontainers to
// it.
func (m *Testcontainers) Setup(ctr *Container) *Container {
	return ctr.
		WithServiceBinding("docker", m.DockerService()).
		WithEnvVariable("DOCKER_HOST", "tcp://docker:2375").
		WithEnvVariable("TESTCONTAINERS_RYUK_DISABLED", "true")
}
