package main

import "main/internal/dagger"

// Compose is an API for using a Docker daemon.
type Daemon struct {
	// The version of Docker to use.
	Version string

	// An optional cache volume to mount at /var/lib/docker.
	Cache *dagger.CacheVolume
}

// WithVersion allows you to specify a Docker version to use.
func (m *Daemon) WithVersion(version string) *Daemon {
	m.Version = version
	return m
}

// WithCache sets a cache volume to mount at /var/lib/docker.
func (m *Daemon) WithCache(cache *dagger.CacheVolume) *Daemon {
	m.Cache = cache
	return m
}

// Service returns a Docker daemon service.
func (m *Daemon) Service() *dagger.Service {
	var image = "docker:dind"
	if m.Version != "" {
		image = "docker:" + m.Version + "-dind"
	}

	ctr := dag.Container().From(image)

	// Dagger brings its own pid 1, so set this to avoid a warning.
	ctr = ctr.WithEnvVariable("TINI_SUBREAPER", "true")

	if m.Cache != nil {
		ctr = ctr.WithMountedCache("/var/lib/docker", m.Cache)
	}

	ctr = ctr.WithExposedPort(2375)

	return ctr.AsService(dagger.ContainerAsServiceOpts{
		Args: []string{
			"dockerd",                   // this appears to be load-bearing
			"--tls=false",               // set a flag explicitly to disable TLS
			"--host=tcp://0.0.0.0:2375", // listen on all interfaces
		},
		InsecureRootCapabilities: true,
	})
}
