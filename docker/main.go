package main

import "main/internal/dagger"

// Docker exposes APIs for using Docker and Docker related tools.
type Docker struct{}

// Daemon returns an API for using a Docker Daemon.
func (m *Docker) Daemon() *Daemon {
	return &Daemon{}
}

// Compose returns an API for using Docker Compose.
func (m *Docker) Compose(
	dir *dagger.Directory,
	// +optional
	// +default=["docker-compose.yml"]
	files []string,
) *Compose {
	return &Compose{
		Dir:   dir,
		Files: files,
	}
}
