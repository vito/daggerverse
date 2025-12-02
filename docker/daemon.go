package main

import (
	"main/internal/dagger"
	"strconv"
	"strings"
)

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

// isDockerV29OrLater checks if the version is 29 or later by comparing major version
func isDockerV29OrLater(version string) bool {
	// Extract major version (e.g., "28.5.1" -> "28", "29" -> "29")
	majorStr := version
	if idx := strings.Index(version, "."); idx > 0 {
		majorStr = version[:idx]
	}

	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return false // If we can't parse, assume an older version
	}

	return major >= 29
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

	// Determine the effective version for conditional logic
	effectiveVersion := m.Version
	if effectiveVersion == "" {
		effectiveVersion = "28"
	}

	// Check if Docker 29 or later for conditional features
	isV29Plus := isDockerV29OrLater(effectiveVersion)

	// Only auto-create cache for Docker 29+ to avoid nested overlay slowdown
	// Docker 28 works fine with overlay2 directly on the container filesystem
	cache := m.Cache
	if cache == nil && isV29Plus {
		cacheKey := "dagger-docker-lib-v" + effectiveVersion
		cache = dag.CacheVolume(cacheKey)
	}

	// Mount cache if specified (either user-provided or auto-created for v29+)
	if cache != nil {
		ctr = ctr.WithMountedCache("/var/lib/docker", cache)
	}

	ctr = ctr.WithExposedPort(2375)

	// Build dockerd args
	args := []string{
		"dockerd",                   // this appears to be load-bearing
		"--tls=false",               // set a flag explicitly to disable TLS
		"--host=tcp://0.0.0.0:2375", // listen on all interfaces
	}

	// Only disable containerd-snapshotter for Docker 29+ (not needed for 28)
	if isV29Plus {
		args = append(args, "--feature", "containerd-snapshotter=false")
	}

	// Add a storage driver if specified
	if m.StorageDriver != "" {
		args = append(args, "--storage-driver="+m.StorageDriver)
	}

	return ctr.AsService(dagger.ContainerAsServiceOpts{
		Args:                     args,
		InsecureRootCapabilities: true,
		UseEntrypoint:            true,
	})
}
