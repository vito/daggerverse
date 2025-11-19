package main

import "main/internal/dagger"

// Compose is an API for using a Docker daemon.
type Daemon struct {
	// The version of Docker to use.
	Version string

	// An optional cache volume to mount at /var/lib/docker.
	Cache *dagger.CacheVolume

	// An optional storage driver to use (e.g., "vfs", "overlay2", "native").
	StorageDriver string
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

// WithStorageDriver allows you to specify a storage driver to use.
func (m *Daemon) WithStorageDriver(driver string) *Daemon {
	m.StorageDriver = driver
	return m
}

// Service returns a Docker daemon service.
func (m *Daemon) Service() *dagger.Service {
	// Default to Docker 28 for stability (Docker 29 has nested overlay issues)
	var image = "docker:28-dind"
	if m.Version != "" {
		image = "docker:" + m.Version + "-dind"
	}

	ctr := dag.Container().From(image)

	// Dagger brings its own pid 1, so set this to avoid a warning.
	ctr = ctr.WithEnvVariable("TINI_SUBREAPER", "true")

	// Always use a cache volume to prevent nested overlay issues
	cache := m.Cache
	if cache == nil {
		// Use version-specific cache key for isolation
		cacheKey := "dagger-docker-lib-v28"
		if m.Version != "" {
			cacheKey = "dagger-docker-lib-v" + m.Version
		}
		cache = dag.CacheVolume(cacheKey)
	}
	ctr = ctr.WithMountedCache("/var/lib/docker", cache)

	ctr = ctr.WithExposedPort(2375)

	// Build dockerd args
	args := []string{
		"dockerd",                   // this appears to be load-bearing
		"--tls=false",               // set a flag explicitly to disable TLS
		"--host=tcp://0.0.0.0:2375", // listen on all interfaces
		"--feature", "containerd-snapshotter=false", // Disable for Docker 29+ compatibility
	}

	// Add storage driver if specified
	if m.StorageDriver != "" {
		args = append(args, "--storage-driver="+m.StorageDriver)
	}

	return ctr.AsService(dagger.ContainerAsServiceOpts{
		Args:                     args,
		InsecureRootCapabilities: true,
		UseEntrypoint:            true,
	})
}
