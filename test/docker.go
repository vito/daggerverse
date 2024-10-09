package main

import "dagger/test/internal/dagger"

func (m *Main) Wordpress() *dagger.Service {
	return dag.Docker().Compose(dag.CurrentModule().Source(), dagger.DockerComposeOpts{
		Files: []string{"wordpress.yml"},
	}).All()
}
