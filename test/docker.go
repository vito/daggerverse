package main

func (m *Main) Wordpress() *Service {
	return dag.Docker().Compose(dag.CurrentModule().Source(), DockerComposeOpts{
		Files: []string{"wordpress.yml"},
	}).All()
}
