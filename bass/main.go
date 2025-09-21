// Bass is a Dagger SDK for the Bass scripting language (https://bass-lang.org).
package main

import (
	"context"
	"fmt"
	"path"
	"testing"

	"dagger/bass/internal/dagger"
	"dagger/bass/internal/telemetry"
	"dagger/bass/runtime"

	"github.com/dagger/testctx"
	"github.com/dagger/testctx/oteltest"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/runtimes"
	"go.opentelemetry.io/otel/trace"
)

const Golang = "golang:1.23"

func New() *BassSdk {
	return &BassSdk{
		RequiredPaths: []string{
			// "**/package.json",
			// "**/package-lock.json",
			// "**/tsconfig.json",
		},
	}
}

type BassSdk struct {
	RequiredPaths []string
}

const (
	ModSourceDirPath         = "/src"
	EntrypointExecutableFile = "/bass"
	EntrypointExecutablePath = "src/" + EntrypointExecutableFile
	codegenBinPath           = "/codegen"
)

// ModuleRuntime returns a container with the node entrypoint ready to be called.
func (t *BassSdk) ModuleRuntime(
	ctx context.Context,
	modSource *dagger.ModuleSource,
	introspectionJson string,
) (*dagger.Container, error) {
	return t.CodegenBase(ctx, modSource, introspectionJson)
}

// Codegen returns the generated API client based on user's module
func (t *BassSdk) Codegen(
	ctx context.Context,
	modSource *dagger.ModuleSource,
	introspectionJson string,
) (*dagger.GeneratedCode, error) {
	ctr, err := t.CodegenBase(ctx, modSource, introspectionJson)
	if err != nil {
		return nil, err
	}

	return dag.GeneratedCode(ctr.Directory(ModSourceDirPath)).
		WithVCSGeneratedPaths([]string{}).
		WithVCSIgnoredPaths([]string{}), nil
}

func (t *BassSdk) CodegenBase(
	ctx context.Context,
	modSource *dagger.ModuleSource,
	introspectionJson string,
) (*dagger.Container, error) {
	modName, err := modSource.ModuleOriginalName(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not load module config: %v", err)
	}

	subPath, err := modSource.SourceSubpath(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not load module config: %v", err)
	}

	modSrcDir := path.Join(ModSourceDirPath, subPath)

	return t.Base().
		WithMountedDirectory(ModSourceDirPath, modSource.ContextDirectory()).
		WithWorkdir(modSrcDir).
		WithEntrypoint([]string{"/bass", modSrcDir, modName}), nil
	// WithNewFile(schemaPath, ContainerWithNewFileOpts{
	// 	Contents: introspectionJson,
	// }).
	// WithExec([]string{
	// 	"--lang", "typescript",
	// 	"--module-context", ModSourceDirPath,
	// 	"--output", genPath,
	// 	"--module-name", name,
	// 	"--introspection-json-path", schemaPath,
	// }, ContainerWithExecOpts{
	// 	ExperimentalPrivilegedNesting: true,
	// }), nil
}

func (t *BassSdk) Base() *dagger.Container {
	return dag.Container().
		From("busybox").
		WithFile("/bass", t.Entrypoint()).
		WithEntrypoint([]string{"/bass"})
}

func (t *BassSdk) Entrypoint() *dagger.File {
	return t.goBase().
		WithExec([]string{"go", "build", "-o", "/bass", "./entrypoint"}).
		File("/bass")
}

func (t *BassSdk) Repl() *dagger.Container {
	return t.Base().
		WithDefaultTerminalCmd([]string{"/bass"}).
		WithMountedCache("/xdg/home", dag.CacheVolume("bass-repl-home")).
		WithEnvVariable("XDG_DATA_HOME", "/xdg/home").
		Terminal(dagger.ContainerTerminalOpts{
			ExperimentalPrivilegedNesting: true,
		})
}

func (t *BassSdk) Test(ctx context.Context) error {
	m := testing.MainStart(TestDeps{}, []testing.InternalTest{
		{
			Name: "TestRuntime",
			F: func(t *testing.T) {
				tc := testctx.New(t,
					testctx.WithParallel(),
					oteltest.WithLogging[*testing.T](oteltest.LogConfig{
						LoggerProvider: telemetry.LoggerProvider(ctx),
					}),
					oteltest.WithTracing[*testing.T](oteltest.TraceConfig{
						TracerProvider: trace.SpanFromContext(ctx).TracerProvider(),
					})).
					WithContext(ctx)
				runtimes.Suite(tc.Context(), tc, bass.RuntimeConfig{
					Platform: bass.LinuxPlatform,
					Runtime:  runtime.Name,
				}, runtimes.SkipSuites(
					"tls.bass",
					"cache-cmd.bass",
					"globs.bass",
				))
			},
		},
	}, nil, nil, nil)

	if m.Run() != 0 {
		return fmt.Errorf("tests failed")
	}

	return nil
}

func (t *BassSdk) goBase() *dagger.Container {
	return dag.Container().From(Golang).
		WithEnvVariable("CGO_ENABLED", "0").
		WithDirectory("/src", dag.CurrentModule().Source()).
		WithWorkdir("/src").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache")
}
