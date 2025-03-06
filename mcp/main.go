// MCP Server Implementation as a Dagger Module
//
// This module provides Dagger functions that implement MCP tools for interacting with
// the Dagger GraphQL API, introspecting the schema, and executing GraphQL queries.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"golang.org/x/sync/errgroup"

	"dagger/mcp-gql/internal/dagger"
	"dagger/mcp-gql/introspection"
	"dagger/mcp-gql/knowledge"
)

type McpDagger struct {
	Modules   []string
	Variables []Variable
}

type Variable struct {
	Name  string
	Value dagger.JSON
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

// DaggerVersion returns the Dagger version
func (m *McpDagger) DaggerVersion(ctx context.Context) (string, error) {
	return dag.Version(ctx)
}

// InstallModule installs a Dagger module into the current schema
func (m *McpDagger) InstallModule(ctx context.Context, ref string) (*McpDagger, error) {
	// Check that the module ref is valid
	err := dag.ModuleSource(ref).AsModule().Serve(ctx)
	if err != nil {
		return nil, err
	}
	m.Modules = append(m.Modules, ref)
	return m, nil
}

// LearnSDK provides guidance on how to convert a GraphQL query to code using a Dagger SDK
func (m *McpDagger) LearnSDK(ctx context.Context, sdk string) (string, error) {
	switch strings.ToLower(sdk) {
	case "go", "golang":
		return knowledge.GoSDK, nil
	default:
		return "", fmt.Errorf("unknown SDK: %s", sdk)
	}
}

// LearnSchema retrieves a snapshot of the current schema in GraphQL SDL format
func (m *McpDagger) LearnSchema(ctx context.Context, typeName string) (string, error) {
	if err := m.serveModules(ctx); err != nil {
		return "", err
	}

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

/*
WithVariable runs a query and stores the unrolled result as a variable for future use.

It is a requirement for any queries that use IDs. You must NEVER pass IDs
around directly.

In Dagger's schema, all Object types have their own corresponding ID type. For
example, `SpokenWord` has an `id: SpokenWordID!` field.

This practice enables any object to be passed as an argument to any other
object, and having separate types for each (unlike typical GraphQL) enforces
type safety for function arguments.

Take special care with ID arguments (`ContainerID`, `FooID`, etc.); they are
too large to display. Instead, use `setVariable` with `run_query` to fetch and
assign the ID to a variable that can be used by future queries.

Let's say I want to pass a `FileID` from one query to another. First, assign
the ID:

	withVariable(
		name: "myFile",
		"""
		query {
		  container {
			withNewFile(path: "/hello.txt", contents: "hi") {
			  file(path: "/hello.txt") {
				id
			  }
			}
		  }
		}
		"""
	)

Then use it in another query:

	runQuery(
		"""
		query UseFile($myFile: FileID!) {
		  container {
			withFile(path: "/copy.txt", source: $myFile) {
			  stdout
			}
		  }
		}
		"""
	)

Repeat this process recursively as necessary.

## Always select scalar fields, not objects

Every query must select scalar fields.

Let's say we have this schema:

	type Query {
	  helloWorld: HelloWorld!
	}

	type HelloWorld {
	  sayHi: SpokenWord!
	}

	type SpokenWord {
	  message: String!
	}

That means that this query does not make sense:

	query {
	  helloWorld {
	    sayHi(arg: "hey")
	  }
	}

The `sayHi` field returns an object type, `SpokenWord!`, so the query is not
valid. Instead, you must select a sub-field:

	query {
	  helloWorld {
	    sayHi(arg: "hey") {
	      message
	    }
	  }
	}
*/
func (m *McpDagger) WithVariable(
	ctx context.Context,
	// The variable name to set, without the $.
	name string,
	// The GraphQL query to execute.
	query string,
) (*McpDagger, error) {
	if err := m.serveModules(ctx); err != nil {
		return nil, err
	}
	result, err := m.RunQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	slog.Info("setting variable", "name", name, "value", result)
	m.Variables = append(m.Variables, Variable{
		Name:  name,
		Value: result,
	})
	return m, nil
}

/*
RunQuery executes a GraphQL query against the Dagger API

Your overall process for querying is this:

 1. Identify any module refs and install them first. They always begin with a
    hostname, most commonly github.com/.
 2. Analyze the schema to ensure you are constructing a wholly valid query.
 3. Only after you are certain the query is correct, run it. Never guess - you
    must have 100% certainty.

Each heading is a firm rule for you to follow throughout this process.

## Only install module refs that have been explicitly mentioned

NEVER guess a module ref. If one isn't mentioned, don't assume it exists.

## Only run queries that you know are valid

Before running any query, first ensure that it is a valid query. Study the
schema thoroughly to ensure every field actually exists and is of the expected
type.

Use the `learn_schema` tool to study the GraphQL schema available to you. Never
guess an API.

Pay close attention to all types referenced by fields. When an argument's type
is non-null (ending with a `!`), that means the argument is required. When it
is nullable, that means the argument is optional.

Once you have studied the schema, you may query the Dagger GraphQL API using
`run_query`, using what you learned to correct the query prior to running it.

## Use sub-selections for chaining

Use standard GraphQL syntax.

In Dagger, field selections are always evaluated in parallel. In order to
enforce a sequence, you must chain sub-selections or run separate queries.

Chaining is the bread and butter of the Dagger API. In GraphQL, this translates
to many nested sub-selections:

# CORRECT:

	query {
	  foo {
	    bar(arg: "one") {
	      baz(anotherArg: 2) {
		stdout
	      }
	    }
	  }
	}

# INCORRECT:

	query {
	  foo {
	    bar(arg: "one")
	    baz(anotherArg: 2) {
	      stdout
	    }
	  }
	}

Most of the Dagger API is pure. Instead of creating a container and mutating
its filesystem, you apply incremental transformations by chaining API calls -
in GraphQL terms, making repeated sub-selections.
*/
func (m *McpDagger) RunQuery(
	ctx context.Context,
	// The GraphQL query to execute.
	query string,
) (dagger.JSON, error) {
	if err := m.serveModules(ctx); err != nil {
		return "", err
	}
	var resp graphql.Response
	if err := dag.GraphQLClient().MakeRequest(ctx, &graphql.Request{
		Query:     query,
		Variables: m.vars(),
	}, &resp); err != nil {
		return "", err
	}
	val := unroll(resp.Data)
	payload, err := json.Marshal(val)
	if err != nil {
		return "", err
	}
	return dagger.JSON(payload), nil
}

func (m *McpDagger) serveModules(ctx context.Context) error {
	eg := new(errgroup.Group)
	for _, ref := range m.Modules {
		ref := ref
		eg.Go(func() error {
			err := dag.ModuleSource(ref).AsModule().Serve(ctx)
			if err != nil {
				return err
			}
			return nil
		})
	}
	return eg.Wait()
}

func (m *McpDagger) vars() map[string]json.RawMessage {
	vars := make(map[string]json.RawMessage)
	for _, v := range m.Variables {
		vars[v.Name] = json.RawMessage(v.Value)
	}
	return vars
}

// Helper function to unroll query results for variable storage
func unroll(val any) any {
	if m, ok := val.(map[string]any); ok {
		for _, v := range m {
			return unroll(v)
		}
		return nil
	} else {
		return val
	}
}
