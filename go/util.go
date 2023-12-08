package main

// GlobalCache sets $GOMODCACHE to /go/pkg/mod and $GOCACHE to /go/build-cache
// and mounts cache volumes to both.
func (g *Go) GlobalCache(ctr *Container) *Container {
	return ctr.
		WithMountedCache("/go/pkg/mod", g.ModCache).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", g.BuildCache).
		WithEnvVariable("GOCACHE", "/go/build-cache")
}

// BinPath sets $GOBIN to /go/bin and prepends it to $PATH.
func (g *Go) BinPath(ctr *Container) *Container {
	return ctr.
		WithEnvVariable("GOBIN", "/go/bin").
		WithEnvVariable("PATH", "$GOBIN:$PATH", ContainerWithEnvVariableOpts{
			Expand: true,
		})
}

func Cd(dst string, src *Directory) WithContainerFunc {
	return func(ctr *Container) *Container {
		return ctr.
			WithMountedDirectory(dst, src).
			WithWorkdir(dst)
	}
}
