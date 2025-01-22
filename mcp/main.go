package main

import (
	"dagger/mcp-gql/internal/dagger"
)

type McpDagger struct {
}

// Returns a container that echoes whatever string argument is provided
func (m *McpDagger) Server() *dagger.Service {
	return dag.
		Wolfi().
		Container().
		WithFile("/bin/server",
			dag.Go(dag.CurrentModule().Source()).Binary("./server/")).
		WithEnvVariable("PORT", "8080").
		WithDefaultArgs([]string{"/bin/server"}).
		WithExposedPort(8080).
		AsService(dagger.ContainerAsServiceOpts{
			ExperimentalPrivilegedNesting: true,
		})
}
