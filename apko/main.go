// Apko builds containers from simple lists of packages.
package main

import (
	"runtime"

	"gopkg.in/yaml.v3"
)

// `apko` is a command-line tool developed by Chainguard
// (https://chainguard.dev) that allows users to build container images using a
// declarative language based on YAML. `apko` is so named as it uses the Alpine
// apk package format and is inspired by the `ko` build tool.
//
// See https://edu.chainguard.dev/open-source/apko/getting-started-with-apko/
// for more information.

type Apko struct{}

// Alpine returns a Container with the specified packages installed from Alpine
// repositories.
func (Apko) Alpine(args struct {
	Packages []string `doc:"List of package names to install." required:"true"`
	Branch   string   `doc:"Alpine branch to use." default:"edge"`
}) (*Container, error) {
	ic := baseConfig()
	ic["contents"] = cfg{
		"repositories": []string{
			"https://dl-cdn.alpinelinux.org/alpine/" + args.Branch + "/main",
		},
		"packages": append([]string{"alpine-base"}, args.Packages...),
	}
	return apko(ic)
}

// Wolfi returns a Container with the specified packages installed from Wolfi
// OS repositories.
func (Apko) Wolfi(packages []string) (*Container, error) {
	ic := baseConfig()
	ic["contents"] = cfg{
		"repositories": []string{
			"https://packages.wolfi.dev/os",
		},
		"keyring": []string{
			"https://packages.wolfi.dev/os/wolfi-signing.rsa.pub",
		},
		"packages": append([]string{"wolfi-base"}, packages...),
	}
	return apko(ic)
}

type cfg map[string]any

func baseConfig() cfg {
	return cfg{
		"cmd": "/bin/sh",
		"environment": cfg{
			"PATH": "/usr/sbin:/sbin:/usr/bin:/bin",
		},
		"archs": []string{runtime.GOARCH},
	}
}

func apko(cfg any) (*Container, error) {
	cfgYAML, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	return dag.Container().Import(
		dag.Container().
			From("cgr.dev/chainguard/apko").
			WithMountedFile(
				"/config.yml",
				dag.Directory().
					WithNewFile("config.yml", string(cfgYAML)).
					File("config.yml"),
			).
			WithDirectory("/layout", dag.Directory()).
			WithMountedCache("/apkache", dag.CacheVolume("apko")).
			WithExec([]string{
				"build",
				"--cache-dir", "/apkache",
				"/config.yml", "latest", "/layout.tar",
			}).
			File("/layout.tar"),
	), nil
}
