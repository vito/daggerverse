package main

func (m *Main) Wordpress() *Service {
	return dag.Docker().Compose(dag.Host().Directory("."), DockerComposeOpts{
		Files: []string{"wordpress.yml"},
	}).All()
}
