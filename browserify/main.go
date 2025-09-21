// Generates a bundle.js file from configured npm packages using Browserify.

package main

import (
	"dagger/browserify/internal/dagger"
	"fmt"
	"strings"
)

type Browserify struct {
	Packages []Package
}

type Package struct {
	Name    string
	Version string
	Binding string
}

// Installs a package that will be globally bound as window.<binding>.
func (m *Browserify) WithPackage(
	name,
	version, /* +optional +default="latest" */
	binding string,
) *Browserify {
	m.Packages = append(m.Packages, Package{
		Name:    name,
		Version: version,
		Binding: binding,
	})
	return m
}

// Generates a bundle.js file from the configured packages.
func (m *Browserify) Bundle() *dagger.File {
	installPackages := []string{"npm", "install"}
	for _, pkg := range m.Packages {
		installPackages = append(installPackages, fmt.Sprintf("%s@%s", pkg.Name, pkg.Version))
	}
	return dag.Apko().Wolfi([]string{"nodejs", "npm"}).
		WithExec([]string{"npm", "install", "-g", "browserify"}).
		WithEnvVariable("PATH", "/usr/local/bin:$PATH", dagger.ContainerWithEnvVariableOpts{
			Expand: true,
		}).
		WithWorkdir("/src").
		WithNewFile("main.js", m.Main()).
		WithExec(installPackages).
		WithExec([]string{"browserify", "main.js", "-o", "bundle.js"}).
		File("bundle.js")
}

// The main.js file passed to browserify.
func (m *Browserify) Main() string {
	main := new(strings.Builder)
	for _, pkg := range m.Packages {
		fmt.Fprintf(main, "global.window.%s = require('%s');\n", pkg.Binding, pkg.Name)
	}
	return main.String()
}
