package main

// Docker exposes APIs for using Docker and Docker related tools.
type Docker struct{}

// Daemon returns an API for using a Docker Daemon.
func (m *Docker) Daemon() *Daemon {
	return &Daemon{}
}

// Compose returns an API for using Docker Compose.
func (m *Docker) Compose(dir *Directory, files Optional[[]string]) *Compose {
	return &Compose{
		Dir:   dir,
		Files: files.GetOr([]string{"docker-compose.yml"}),
	}
}
