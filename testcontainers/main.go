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
		return defaultDind()
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

func defaultDind() *Service {
	return dag.Container().From("docker:dind").
		WithFocus().
		WithEnvVariable("TINI_SUBREAPER", "").
		WithExec([]string{
			"dockerd",                   // this appears to be load-bearing
			"--tls=false",               // set a flag explicitly to disable TLS
			"--host=tcp://0.0.0.0:2375", // listen on all interfaces
		}, ContainerWithExecOpts{
			InsecureRootCapabilities: true,
		}).
		WithExposedPort(2375).
		AsService()
}
