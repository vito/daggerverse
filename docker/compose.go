package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"main/internal/dagger"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
)

// Compose is an API for using Docker Compose.
type Compose struct {
	// The directory to use as the context for the Compose project.
	Dir *dagger.Directory

	// The Compose config files to use, within the directory.
	Files []string

	// Environment variables to interpolate into the Compose config files.
	Env []EnvVar
}

// EnvVar represents an environment variable to interpolate into the Compose config
// files.
type EnvVar struct {
	Name  string
	Value string
}

// WithEnv sets an environment variable that may be interpolated into the
// Compose config files.
func (m *Compose) WithEnv(name, val string) *Compose {
	m.Env = append(m.Env, EnvVar{
		Name:  name,
		Value: val,
	})
	return m
}

// All returns a proxy service that forwards traffic to all defined services.
func (m *Compose) All(ctx context.Context) (*dagger.Service, error) {
	env := make(types.Mapping)
	for _, e := range m.Env {
		env[e.Name] = e.Value
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	loaderConfig := types.ConfigDetails{
		Version:     "3",
		WorkingDir:  wd,
		Environment: env,
	}

	for _, f := range m.Files {
		content, err := m.Dir.File(f).Contents(ctx)
		if err != nil {
			return nil, err
		}
		loaderConfig.ConfigFiles = append(loaderConfig.ConfigFiles, types.ConfigFile{
			Filename: filepath.Base(f),
			Content:  []byte(content),
		})
	}

	project, err := loader.LoadWithContext(
		ctx,
		loaderConfig,
		func(options *loader.Options) {
			options.SetProjectName("dagger-compose", true)
		},
	)
	if err != nil {
		return nil, err
	}

	proxy := dag.Proxy()

	for _, composeSvc := range project.Services {
		svc, err := m.convert(project, composeSvc)
		if err != nil {
			return nil, err
		}
		for _, port := range composeSvc.Ports {
			frontend, err := strconv.Atoi(port.Published)
			if err != nil {
				return nil, err
			}
			switch port.Mode {
			case "ingress":
				proxy = proxy.WithService(
					svc,
					composeSvc.Name,
					frontend,
					int(port.Target),
				)
			default:
				return nil, fmt.Errorf("port mode %s not supported", port.Mode)
			}
		}
	}

	return proxy.Service(), nil
}

func (m *Compose) convert(project *types.Project, svc types.ServiceConfig) (*dagger.Service, error) {
	ctr := dag.Container()

	if svc.Image != "" {
		ctr = ctr.From(svc.Image)
	} else if svc.Build != nil {
		args := []dagger.BuildArg{}
		for name, val := range svc.Build.Args {
			if val != nil {
				args = append(args, dagger.BuildArg{
					Name:  name,
					Value: *val,
				})
			}
		}

		ctr = ctr.Build(m.Dir.Directory(svc.Build.Context), dagger.ContainerBuildOpts{
			Dockerfile: svc.Build.Dockerfile,
			BuildArgs:  args,
			Target:     svc.Build.Target,
		})
	}

	// sort env to ensure same container
	type env struct{ name, value string }
	envs := []env{}
	for name, val := range svc.Environment {
		if val != nil {
			envs = append(envs, env{name, *val})
		}
	}
	sort.Slice(envs, func(i, j int) bool {
		return envs[i].name < envs[j].name
	})
	for _, env := range envs {
		ctr = ctr.WithEnvVariable(env.name, env.value)
	}

	for _, port := range svc.Ports {
		switch port.Mode {
		case "ingress":
			protocol := dagger.NetworkProtocolTcp
			switch port.Protocol {
			case "udp":
				protocol = dagger.NetworkProtocolUdp
			case "", "tcp":
				protocol = dagger.NetworkProtocolTcp
			default:
				return nil, fmt.Errorf("protocol %s not supported", port.Protocol)
			}

			ctr = ctr.WithExposedPort(int(port.Target), dagger.ContainerWithExposedPortOpts{
				Protocol: protocol,
			})
		default:
			return nil, fmt.Errorf("port mode %s not supported", port.Mode)
		}
	}

	for _, expose := range svc.Expose {
		port, err := strconv.Atoi(expose)
		if err != nil {
			return nil, err
		}

		ctr = ctr.WithExposedPort(port)
	}

	for _, vol := range svc.Volumes {
		switch vol.Type {
		case types.VolumeTypeBind:
			ctr = ctr.WithMountedDirectory(vol.Target, m.Dir.Directory(vol.Source))
		case types.VolumeTypeVolume:
			ctr = ctr.WithMountedCache(vol.Target, dag.CacheVolume(vol.Source))
		default:
			return nil, fmt.Errorf("volume type %s not supported", vol.Type)
		}
	}

	for depName := range svc.DependsOn {
		cfg, err := project.GetService(depName)
		if err != nil {
			return nil, err
		}

		svc, err := m.convert(project, cfg)
		if err != nil {
			return nil, err
		}

		ctr = ctr.WithServiceBinding(depName, svc)
	}

	if !svc.Entrypoint.IsZero() {
		ctr = ctr.WithEntrypoint(svc.Entrypoint)
	}

	opts := dagger.ContainerWithExecOpts{
		UseEntrypoint: true,
	}

	if svc.Privileged {
		opts.InsecureRootCapabilities = true
	}

	ctr = ctr.WithExec(svc.Command, opts)

	return ctr.AsService(), nil
}
