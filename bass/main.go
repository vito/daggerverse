// Bass is a Dagger SDK for the Bass scripting language (https://bass-lang.org).
package main

import (
	"context"
	"fmt"
	"path"
)

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
func (t *BassSdk) ModuleRuntime(ctx context.Context, modSource *ModuleSource, introspectionJson string) (*Container, error) {
	return t.CodegenBase(ctx, modSource, introspectionJson)
}

// Codegen returns the generated API client based on user's module
func (t *BassSdk) Codegen(ctx context.Context, modSource *ModuleSource, introspectionJson string) (*GeneratedCode, error) {
	ctr, err := t.CodegenBase(ctx, modSource, introspectionJson)
	if err != nil {
		return nil, err
	}

	return dag.GeneratedCode(ctr.Directory(ModSourceDirPath)).
		WithVCSGeneratedPaths([]string{}).
		WithVCSIgnoredPaths([]string{}), nil
}

func (t *BassSdk) CodegenBase(ctx context.Context, modSource *ModuleSource, introspectionJson string) (*Container, error) {
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

func (t *BassSdk) Base() *Container {
	return dag.Container().
		WithFile("/bass", t.Entrypoint()).
		WithEntrypoint([]string{"/bass"})
}

func (t *BassSdk) Entrypoint() *File {
	return dag.Container().From("golang:1.22").
		WithEnvVariable("CGO_ENABLED", "0").
		WithDirectory("/src", dag.CurrentModule().Source()).
		WithWorkdir("/src").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithExec([]string{"go", "build", "-o", "/bass", "./entrypoint"}).
		File("/bass")
}
