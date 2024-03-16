// High-level interfaces for building and testing Go code.

package main

import (
	"fmt"
	"strings"
)

type Go struct {
	Base       *Container
	ModCache   *CacheVolume
	BuildCache *CacheVolume
}

func New(
	// +optional
	base *Container,
	// +optional
	modCache *CacheVolume,
	// +optional
	buildCache *CacheVolume,
) *Go {
	if base == nil {
		base = dag.Container().From("golang:1")
	}
	if modCache == nil {
		modCache = dag.CacheVolume("go-mod")
	}
	if buildCache == nil {
		buildCache = dag.CacheVolume("go-build")
	}
	return &Go{
		Base:       base,
		ModCache:   modCache,
		BuildCache: buildCache,
	}
}

// FromVersion sets the base image to the given Go version.
func (g *Go) FromVersion(version string) *Go {
	g.Base = g.Base.From("golang:" + version)
	return g
}

// Build builds Go code using the go build CLI.
func (g *Go) Build(
	// The directory containing code to build.
	src *Directory,
	// Packages to build.
	// +optional
	packages []string,
	// Subdirectory in which to place the built artifacts.
	// +optional
	subdir string,
	// -X definitions to pass to go build -ldflags.
	// +optional
	xDefs []string,
	// Whether to enable CGO.
	// +optional
	static bool,
	// Whether to build with race detection.
	// +optional
	race bool,
	// GOOS to pass to go build for cross-compilation.
	// +optional
	GOOS string,
	// GOARCH to pass to go build. for cross-compilation
	// +optional
	GOARCH string,
	// Arbitrary flags to pass along to go build.
	// +optional
	buildFlags []string,
) *Directory {
	ctr := g.Base.
		With(g.GlobalCache).
		WithDirectory("/out", dag.Directory()).
		With(Cd("/src", src))

	if static {
		ctr = ctr.WithEnvVariable("CGO_ENABLED", "0")
	}

	if GOOS != "" {
		ctr = ctr.WithEnvVariable("GOOS", GOOS)
	}

	if GOARCH != "" {
		ctr = ctr.WithEnvVariable("GOARCH", GOARCH)
	}

	cmd := []string{
		"go", "build",
		"-o", "/out/",
		"-trimpath", // unconditional for reproducible builds
	}

	if race {
		cmd = append(cmd, "-race")
	}

	cmd = append(cmd, buildFlags...)

	if len(xDefs) > 0 {
		cmd = append(cmd, "-ldflags", "-X "+strings.Join(xDefs, " -X "))
	}

	cmd = append(cmd, packages...)

	out := ctr.
		WithExec(cmd).
		Directory("/out")

	if subdir != "" {
		out = dag.Directory().WithDirectory(subdir, out)
	}

	return out
}

// Test runs tests using the go test CLI.
func (g *Go) Test(
	// The directory containing code to test.
	src *Directory,
	// Subdirectory in which to run the tests, i.e. go run -C.
	//
	// This is useful when running tests in a Go module that refers to a parent
	// module.
	//
	// +optional
	subdir string,
	// Packages to test.
	// +optional
	packages []string,
	// Run with -v.
	// +optional
	verbose bool,
	// Whether to run tests with race detection.
	// +optional
	race bool,
	// Arbitrary flags to pass along to go test.
	// +optional
	testFlags []string,
	// Whether to run tests insecurely, i.e. with special privileges.
	// +optional
	insecureRootCapabilities bool,
	// Enable experimental Dagger nesting.
	// +optional
	nest bool,
) (*Container, error) {
	ctr := g.Base.
		With(g.GlobalCache).
		WithMountedDirectory("/src", src).
		WithWorkdir("/src")

	pkgs := packages
	if len(pkgs) == 0 {
		pkgs = []string{"./..."}
	}

	goTest := []string{"go", "test"}

	if subdir != "" {
		goTest = append(goTest, "-C", subdir)
	}

	if race {
		goTest = append(goTest, "-race")
	}

	if verbose {
		goTest = append(goTest, "-v")
	}

	goTest = append(goTest, testFlags...)

	goTest = append(goTest, pkgs...)

	return ctr.WithExec(goTest, ContainerWithExecOpts{
		InsecureRootCapabilities:      insecureRootCapabilities,
		ExperimentalPrivilegedNesting: nest,
	}), nil
}

// Gotestsum runs tests using the gotestsum CLI.
//
// The base container must have the gotestsum CLI installed.
func (g *Go) Gotestsum(
	// The directory containing code to test.
	src *Directory,
	// Packages to test.
	// +optional
	packages []string,
	// Gotestsum format to display.
	// +optional
	// +default="testname"
	format string,
	// Whether to run tests with race detection.
	// +optional
	race bool,
	// Whether to run tests insecurely, i.e. with special privileges.
	// +optional
	insecureRootCapabilities bool,
	// Enable experimental Dagger nesting.
	// +optional
	nest bool,
	// Arbitrary flags to pass along to go test.
	// +optional
	goTestFlags []string,
	// Arbitrary flags to pass along to gotestsum.
	// +optional
	gotestsumFlags []string,
) *Container {
	cmd := []string{
		"gotestsum",
		"--no-color=false", // force color
		"--format=" + format,
	}
	cmd = append(cmd, gotestsumFlags...)
	if race {
		goTestFlags = append(goTestFlags, "-race")
	}
	if len(packages) > 0 {
		goTestFlags = append(goTestFlags, packages...)
	}
	if len(goTestFlags) > 0 {
		cmd = append(cmd, "--")
		cmd = append(cmd, goTestFlags...)
	}
	return g.Base.
		With(g.GlobalCache).
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		WithExec(cmd, ContainerWithExecOpts{
			InsecureRootCapabilities:      insecureRootCapabilities,
			ExperimentalPrivilegedNesting: nest,
		})
}

// Generate runs go generate ./... and returns the updated directory.
func (g *Go) Generate(src *Directory) *Directory {
	return g.Base.
		With(g.GlobalCache).
		With(Cd("/src", src)).
		WithExec([]string{"go", "generate", "./..."}).
		Directory("/src")
}

// GolangCILint runs golangci-lint.
//
// The base container must have the golangci-lint CLI installed.
func (g *Go) GolangCILint(
	src *Directory,
	// +optional
	verbose bool,
	// +optional
	timeoutInSeconds int,
) *Container {
	cmd := []string{"golangci-lint", "run"}
	if verbose {
		cmd = append(cmd, "--verbose")
	}
	if timeoutInSeconds > 0 {
		cmd = append(cmd, fmt.Sprintf("--timeout=%ds", timeoutInSeconds))
	}
	return g.Base.
		With(g.GlobalCache).
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		WithExec(cmd)
}
