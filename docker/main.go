package main

type Docker struct{}

func (m *Docker) Daemon() *Daemon {
	return &Daemon{}
}

func (m *Docker) Compose(dir *Directory, file Optional[[]string]) *Compose {
	return &Compose{
		Dir:   dir,
		Files: file.GetOr([]string{"docker-compose.yml"}),
	}
}
