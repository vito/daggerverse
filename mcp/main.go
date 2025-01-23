package main

import (
	"context"
	"strings"

	"github.com/Khan/genqlient/graphql"

	"dagger/mcp-gql/internal/dagger"
	"dagger/mcp-gql/introspection"
)

type McpDagger struct {
}

// The MCP server.
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

// Test out the SDL rendering by dumping the current schema.
func (m *McpDagger) Schema(ctx context.Context, typeName string) (string, error) {
	var resp introspection.Response
	if err := dag.GraphQLClient().MakeRequest(ctx, &graphql.Request{
		Query: introspection.Query,
	}, &graphql.Response{
		Data: &resp,
	}); err != nil {
		return "", err
	}

	resp.Schema.OnlyType(typeName)

	var buf strings.Builder
	resp.Schema.RenderSDL(&buf)
	return buf.String(), nil
}
