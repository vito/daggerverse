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

// WithDockerVersion configures the Docker daemon version to use.
// If not specified, defaults to Docker 28 for stability.
func (m *Testcontainers) WithDockerVersion(version string) *Testcontainers {
	m.Docker = dag.Docker().Daemon().WithVersion(version).Service()
	return m
}

// WithDockerCache configures a cache volume for the Docker daemon.
// If not specified, a default cache volume is automatically created.
func (m *Testcontainers) WithDockerCache(cache *CacheVolume) *Testcontainers {
	m.Docker = dag.Docker().Daemon().WithCache(cache).Service()
	return m
}

// WithDockerStorageDriver configures the storage driver for the Docker daemon.
// Common options: "vfs", "overlay2", "native".
func (m *Testcontainers) WithDockerStorageDriver(driver string) *Testcontainers {
	m.Docker = dag.Docker().Daemon().WithStorageDriver(driver).Service()
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
