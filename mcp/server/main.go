package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"dagger/mcp-gql/internal/dagger"
	"dagger/mcp-gql/introspection"
	"dagger/mcp-gql/knowledge"
)

var PORT = os.Getenv("PORT")

var dag = dagger.Connect()

func init() {
	if PORT == "" {
		PORT = "8080"
	}
}

func main() {
	// ctx := context.Background()
	// ctx = telemetry.InitEmbedded(ctx, nil)
	// defer telemetry.Close()

	s := server.NewMCPServer("Dagger", "0.0.1")

	s.AddTool(
		mcp.NewTool("dagger_version",
			mcp.WithDescription("Print the Dagger version."),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, err := dag.Version(ctx)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(v), nil
		})

	s.AddTool(
		mcp.NewTool("install_module",
			mcp.WithDescription("Install a Dagger module into the current schema."),
			mcp.WithString("ref",
				mcp.Required(),
				mcp.Description("Module ref string to install."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ref, ok := request.Params.Arguments["ref"].(string)
			if !ok {
				return mcp.NewToolResultError("ref must be a string"), nil
			}

			err := dag.ModuleSource(ref).AsModule().Serve(ctx)
			if err != nil {
				return nil, err
			}

			return mcp.NewToolResultText("Module has been installed."), nil
		})

	vars := map[string]any{}

	s.AddTool(
		mcp.NewTool("learn_sdk",
			mcp.WithDescription(
				`Learn how to convert a GraphQL query to code using a Dagger SDK.`,
			),
			mcp.WithString("sdk",
				mcp.Required(),
				mcp.Description("The SDK to learn."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			sdk, ok := request.Params.Arguments["sdk"].(string)
			if !ok {
				return mcp.NewToolResultError("sdk must be a string"), nil
			}
			switch strings.ToLower(sdk) {
			case "go", "golang":
				return mcp.NewToolResultText(knowledge.GoSDK), nil
			default:
				return nil, fmt.Errorf("unknown SDK: %s", sdk)
			}
		})

	s.AddTool(
		mcp.NewTool("learn_schema",
			mcp.WithDescription(
				`Retrieve a snapshot of the current schema in GraphQL SDL format.`,
			),
			mcp.WithString("type",
				mcp.Description("The type to learn about. Start with Query and work from there."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			typeName, ok := request.Params.Arguments["type"].(string)
			if !ok {
				return mcp.NewToolResultError("type must be a string"), nil
			}

			var resp introspection.Response
			if err := dag.GraphQLClient().MakeRequest(ctx, &graphql.Request{
				Query: introspection.Query,
			}, &graphql.Response{
				Data: &resp,
			}); err != nil {
				return nil, err
			}

			resp.Schema.OnlyType(typeName)

			var buf strings.Builder
			resp.Schema.RenderSDL(&buf)
			return mcp.NewToolResultText(buf.String()), nil
		})

	s.AddTool(
		mcp.NewTool("run_query",
			mcp.WithDescription(
				knowledge.Querying,
			),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("The GraphQL query to execute."),
			),
			mcp.WithString("setVariable",
				mcp.Description("Assign the unrolled result value as a GraphQL variable for future queries."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, ok := request.Params.Arguments["query"].(string)
			if !ok {
				return mcp.NewToolResultError("query must be a string"), nil
			}

			var resp graphql.Response
			if err := dag.GraphQLClient().MakeRequest(ctx, &graphql.Request{
				Query:     query,
				Variables: vars,
			}, &resp); err != nil {
				return nil, err
			}
			payload, err := json.Marshal(resp)
			if err != nil {
				return nil, err
			}

			if name, ok := request.Params.Arguments["setVariable"].(string); ok {
				val := unroll(resp.Data)
				slog.Info("setting variable", "name", name, "value", val)
				vars[name] = val
				return mcp.NewToolResultText("Variable defined: $" + name), nil
			}

			return mcp.NewToolResultText(string(payload)), nil
		})

	sseSrv := server.NewSSEServer(s, fmt.Sprintf("http://localhost:%s", PORT))
	if err := sseSrv.Start(fmt.Sprintf("0.0.0.0:%s", PORT)); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

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
