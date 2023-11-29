package main

type Docker struct{}

func (m *Docker) Daemon() *Daemon {
	return &Daemon{}
}

func (m *Docker) Compose(dir *Directory, files Optional[[]string]) *Compose {
	return &Compose{
		Dir:   dir,
		Files: files.GetOr([]string{"docker-compose.yml"}),
	}
}
