// Concourse is a continuous thing-doer.

package main

import (
	"concourse/internal/dagger"
	"concourse/internal/telemetry"
	"context"
	"encoding/json"
	"fmt"
	logpkg "log"
	"os"
	"strings"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/configvalidate"
	"github.com/concourse/concourse/vars"
	"gopkg.in/yaml.v2"
)

func init() {
	atc.EnableAcrossStep = true
}

type Concourse struct {
	Container *dagger.Container
	Postgres  *dagger.Container
	StateTag  string
	WebPort   int
	Runtime   string

	ResourceTypes []ResourceType

	Vars       []*Var
	SecretVars []*SecretVar
}

func New(
	// The image to use for the Concourse container.
	// +optional
	// +default="concourse/concourse:latest"
	image string,
	// The Concourse container.
	// +optional
	container *dagger.Container,
	// The Postgres image to use for the database.
	// +optional
	// +default="postgres:latest"
	postgresImage string,
	// The Postgres container to use for the database.
	// +optional
	postgres *dagger.Container,
	// A tag to use for the state of the Concourse cluster.
	// +optional
	stateTag string,
	// The port to expose for the web node.
	// +optional
	// +default=8080
	port int,
	// The runtime to use on the worker nodes.
	// +optional
	// +default="containerd"
	runtime string,
) *Concourse {
	if container == nil {
		container = dag.Container().From(image)
	}
	if postgres == nil {
		postgres = dag.Container().From(postgresImage)
	}
	return &Concourse{
		Container: container,
		Postgres:  postgres,
		StateTag:  stateTag,
		WebPort:   port,
		Runtime:   runtime,
		// TODO this is pretty slow
		ResourceTypes: []ResourceType{
			{
				Name:      "git",
				Container: dag.Container().From("concourse/git-resource"),
			},
			{
				Name:      "registry-image",
				Container: dag.Container().From("concourse/registry-image-resource"),
			},
			{
				Name:      "time",
				Container: dag.Container().From("concourse/time-resource"),
			},
			{
				Name:      "s3",
				Container: dag.Container().From("concourse/s3-resource"),
			},
			{
				Name:      "semver",
				Container: dag.Container().From("concourse/semver-resource"),
			},
			{
				Name:      "docker-image",
				Container: dag.Container().From("concourse/docker-image-resource"),
			},
			{
				Name:      "github-release",
				Container: dag.Container().From("concourse/github-release-resource"),
			},
			{
				Name:      "bosh-io-release",
				Container: dag.Container().From("concourse/bosh-io-release-resource"),
			},
			{
				Name:      "bosh-io-stemcell",
				Container: dag.Container().From("concourse/bosh-io-stemcell-resource"),
			},
		},
	}
}

// Runs an all-in-one Concourse cluster.
func (m *Concourse) Quickstart() *dagger.Service {
	workerWorkDir := dag.CacheVolume(fmt.Sprintf("concourse-worker-work-dir-%s", m.StateTag))

	return m.Container.
		WithMountedCache("/concourse-worker-work-dir", workerWorkDir).
		WithEnvVariable("CONCOURSE_WORKER_WORK_DIR", "/concourse-worker-work-dir").
		WithServiceBinding("db", m.Database()).
		WithExposedPort(8080).
		WithEnvVariable("CONCOURSE_BIND_PORT", fmt.Sprintf("%d", m.WebPort)).
		WithEnvVariable("CONCOURSE_POSTGRES_HOST", "db").
		WithEnvVariable("CONCOURSE_POSTGRES_DATABASE", "concourse").
		WithEnvVariable("CONCOURSE_POSTGRES_USER", "concourse").
		WithEnvVariable("CONCOURSE_POSTGRES_PASSWORD", "concourse").
		WithEnvVariable("CONCOURSE_ADD_LOCAL_USER", "dagger:dagger").
		WithEnvVariable("CONCOURSE_MAIN_TEAM_LOCAL_USER", "dagger").
		WithEnvVariable("CONCOURSE_CLUSTER_NAME", "dagger").
		WithEnvVariable("CONCOURSE_WORKER_RUNTIME", m.Runtime).
		WithEnvVariable("CONCOURSE_WORKER_BAGGAGECLAIM_DRIVER", "overlay").
		WithEnvVariable("CONCOURSE_ENABLE_PIPELINE_INSTANCES", "true").
		WithEnvVariable("CONCOURSE_ENABLE_ACROSS_STEP", "true").
		WithEnvVariable("CONCOURSE_EXTERNAL_URL", fmt.Sprintf("http://localhost:%d", m.WebPort)).
		WithEntrypoint(nil).
		WithExec([]string{"/usr/local/bin/entrypoint.sh", "quickstart"}, dagger.ContainerWithExecOpts{
			InsecureRootCapabilities: true,
		}).
		AsService()
}

// Runs the Concourse database.
func (m *Concourse) Database() *dagger.Service {
	return m.Postgres.
		WithExposedPort(5432).
		WithEnvVariable("POSTGRES_DB", "concourse").
		WithEnvVariable("POSTGRES_USER", "concourse").
		WithEnvVariable("POSTGRES_PASSWORD", "concourse").
		WithEnvVariable("PGDATA", "/database").
		WithMountedCache("/database", dag.CacheVolume(fmt.Sprintf("concourse-db-%s", m.StateTag))).
		AsService()
}

// Initialize a resource. Resources represent external versioned assets.
//
// Resources are implemented as a container that implements the Concourse
// resource type interface.
//
// See https://concourse-ci.org/implementing-resource-types.html for more
// information.
func (m *Concourse) Resource(name string, container *dagger.Container, source dagger.JSON) *Resource {
	return &Resource{
		Concourse: m,
		Name:      name,
		Container: container,
		Source:    source,
	}
}

// A secret to use for a Concourse config ((variable)).
type SecretVar struct {
	Name  string
	Value *dagger.Secret
}

// Adds a secret to use for a Concourse config ((variable)).
func (m Concourse) WithSecretVar(name string, value *dagger.Secret) *Concourse {
	m.SecretVars = append(m.SecretVars, &SecretVar{
		Name:  name,
		Value: value,
	})
	return &m
}

// A static value to use for a Concourse config ((variable)).
type Var struct {
	Name  string
	Value dagger.JSON
}

// Adds a variable to use for a Concourse config.
//
// See https://concourse-ci.org/vars.html for more information.
func (m Concourse) WithVar(name string, value dagger.JSON) *Concourse {
	m.Vars = append(m.Vars, &Var{
		Name:  name,
		Value: value,
	})
	return &m
}

func (m *Concourse) Interpolate(ctx context.Context, config string) (string, error) {
	staticVars := vars.StaticVariables{}
	for _, secret := range m.SecretVars {
		plaintext, err := secret.Value.Plaintext(ctx)
		if err != nil {
			return "", fmt.Errorf("get plaintext for %s: %w", secret.Name, err)
		}
		staticVars[secret.Name] = plaintext
	}

	for _, v := range m.Vars {
		var val any
		if err := json.Unmarshal([]byte(v.Value), &val); err != nil {
			return "", fmt.Errorf("unmarshal var %s: %w", v.Name, err)
		}
		staticVars[v.Name] = val
	}

	resolver := vars.NewTemplateResolver([]byte(config), []vars.Variables{staticVars})
	resolved, err := resolver.Resolve(true, false)
	if err != nil {
		return "", fmt.Errorf("resolve: %w", err)
	}

	// HACK: we get YAML out of this son of a gun, but we want JSON half the
	// time, but JSON is YAML, so let's just swap it out for JSON
	var ugh any
	if err := yaml.Unmarshal(resolved, &ugh); err != nil {
		return "", fmt.Errorf("unmarshal resolved: %w", err)
	}

	resolvedJSON, err := json.Marshal(itsSymbolsVsStringKeysAllOverAgain(ugh))
	if err != nil {
		return "", fmt.Errorf("marshal resolved: %w", err)
	}

	return string(resolvedJSON), nil
}

func itsSymbolsVsStringKeysAllOverAgain(whee any) any {
	switch v := whee.(type) {
	case map[any]any:
		m := map[string]any{}
		for k, v := range v {
			m[k.(string)] = itsSymbolsVsStringKeysAllOverAgain(v)
		}
		return m
	case []any:
		for i, e := range v {
			v[i] = itsSymbolsVsStringKeysAllOverAgain(e)
		}
	}
	return whee
}

// Load a pipeline configuration from a YAML configuration.
//
// See https://concourse-ci.org/pipelines.html for more information.
func (m *Concourse) LoadPipeline(ctx context.Context, configFile *dagger.File) (*Pipeline, error) {
	config, err := configFile.Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	slog.Info("loading & validating config...")
	var cfg atc.Config
	if err := atc.UnmarshalConfig([]byte(config), &cfg); err != nil {
		return nil, fmt.Errorf("malformed config: %w", err)
	}
	warnings, errMsgs := configvalidate.Validate(cfg)
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, "WARNING:", warning.Message)
	}
	if len(errMsgs) > 0 {
		for _, e := range errMsgs {
			fmt.Fprintln(os.Stderr, "ERROR:", e)
		}
		return nil, fmt.Errorf("invalid pipeline")
	}

	pipeline := &Pipeline{
		Concourse:     m,
		ResourceTypes: m.ResourceTypes,
	}

	for _, resourceType := range cfg.ResourceTypes {
		slog.Info("installing resource type", "name", resourceType.Name)
		ctr, err := pipeline.imageResource(ctx, resourceType.Type, resourceType.Source, resourceType.Params)
		if err != nil {
			return nil, fmt.Errorf("install resource type %s: %w", resourceType.Name, err)
		}
		pipeline.ResourceTypes = append(pipeline.ResourceTypes, ResourceType{
			Name:      resourceType.Name,
			Container: ctr,
		})
	}

	slog.Info("loading resources...")
	for _, resource := range cfg.Resources {
		src, err := json.Marshal(resource.Source)
		if err != nil {
			return nil, err
		}
		resourceType := pipeline.ResourceType(resource.Type)
		if resourceType == nil {
			return nil, fmt.Errorf("unknown resource type: %s", resource.Type)
		}
		// pipeline = pipeline.WithResource(m.Resource(resource.Name, resourceType.Container, JSON(src)))
		pipeline.Resources = append(pipeline.Resources, Resource{
			Name:      resource.Name,
			Container: resourceType.Container,
			Source:    dagger.JSON(src),
		})
	}

	slog.Info("loading jobs...")
	for _, job := range cfg.Jobs {
		cfgJSON, err := json.Marshal(job)
		if err != nil {
			return nil, err
		}
		// pipeline.Jobs[job.Name] = JSON(cfgJSON)
		pipeline.Jobs = append(pipeline.Jobs, Job{
			Name:   job.Name,
			Config: dagger.JSON(cfgJSON),
		})
	}

	return pipeline, nil
}

func (pl *Pipeline) imageResource(ctx context.Context, resourceType string, source atc.Source, params atc.Params) (*dagger.Container, error) {
	baseType := pl.ResourceType(resourceType)
	srcJSON, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	imageResource := pl.Concourse.Resource(resourceType, baseType.Container, dagger.JSON(srcJSON))
	ver, err := imageResource.LatestVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("check resource type %s image: %w", resourceType, err)
	}
	dir, err := ver.Get(ctx, dagger.JSON(paramsJSON))
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}
	return pl.fetchedImage(ctx, dir)
}

func (pl *Pipeline) fetchedImage(ctx context.Context, dir *dagger.Directory) (*dagger.Container, error) {
	ctr := dag.Container().WithRootfs(dir.Directory("rootfs"))
	metadataJSON, err := dir.File("metadata.json").Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("read image metadata: %w", err)
	}
	var imageMeta struct {
		User string   `json:"user"`
		Env  []string `json:"env"`
	}
	if err := json.Unmarshal([]byte(metadataJSON), &imageMeta); err != nil {
		return nil, fmt.Errorf("unmarshal image metadata: %w", err)
	}
	if imageMeta.User != "" {
		ctr = ctr.WithUser(imageMeta.User)
	}
	for _, env := range imageMeta.Env {
		name, val, _ := strings.Cut(env, "=")
		ctr = ctr.WithEnvVariable(name, val)
	}
	return ctr, nil
}

type ResourceType struct {
	// Must be nil when installed onto a Pipeline.
	// +private
	Pipeline *Pipeline

	Name      string
	Container *dagger.Container
	// Name       string      `json:"name"`
	// Type       string      `json:"type"`
	// Source     Source      `json:"source"`
	// Defaults   Source      `json:"defaults,omitempty"`
	// Privileged bool        `json:"privileged,omitempty"`
	// CheckEvery *CheckEvery `json:"check_every,omitempty"`
	// Tags       Tags        `json:"tags,omitempty"`
	// Params     Params      `json:"params,omitempty"`
}

func (m *Pipeline) ResourceType(name string) *ResourceType {
	for _, rt := range m.ResourceTypes {
		if rt.Name == name {
			rt.Pipeline = m
			return &rt
		}
	}
	return nil
}

// Add a resource type to the pipeline.
// func (pl *Pipeline) WithResourceType(resourceType *ResourceType) *Pipeline {
// 	pl.ResourceTypes = append(pl.ResourceTypes, resourceType)
// 	return pl
// }

// Jobs configure a plan for interacting with resources and running tasks.
//
// See https://concourse-ci.org/jobs.html for more information.
type Job struct {
	// This field must be nil when installed on a Pipeline.
	// +private
	Pipeline *Pipeline

	Name   string
	Config dagger.JSON
}

func NewJob(conc *Concourse, pl *Pipeline, name string, config dagger.JSON) *Job {
	return &Job{
		Pipeline: pl,
		Name:     name,
		Config:   config,
	}
}

func (pl *Pipeline) Job(name string) *Job {
	for _, job := range pl.Jobs {
		if job.Name == name {
			job.Pipeline = pl
			return &job
		}
	}
	return nil
}

func (job *Job) Run(ctx context.Context) error {
	var cfg atc.JobConfig
	if err := json.Unmarshal([]byte(job.Config), &cfg); err != nil {
		return err
	}
	build := job.Pipeline.build(ctx)
	step := cfg.StepConfig()
	return step.Visit(build)
}

func (m *Pipeline) Resource(name string) *Resource {
	for _, rt := range m.Resources {
		if rt.Name == name {
			rt.Concourse = m.Concourse
			return &rt
		}
	}
	return nil
}

const checkInterval = time.Minute

// Check for new versions of a resource.
func (r *Resource) Check(
	ctx context.Context,
	// Check from this version. If not specified, only the latest version is returned.
	from dagger.JSON, // +optional
) (vs []*ResourceVersion, rerr error) {
	ctx, span := Tracer().Start(ctx, "check: "+r.Name)
	defer telemetry.End(span, func() error { return rerr })

	sourceJSON, err := r.Concourse.Interpolate(ctx, string(r.Source))
	if err != nil {
		return nil, fmt.Errorf("interpolate resource vars: %w", err)
	}
	req := map[string]any{
		"source": json.RawMessage(sourceJSON),
	}
	if from != "" {
		req["version"] = json.RawMessage(from)
	}
	reqPayload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	ctr := r.Container.WithEnvVariable("NOW", time.Now().Truncate(checkInterval).String())
	stdout, err := ctr.WithExec([]string{"/opt/resource/check"}, dagger.ContainerWithExecOpts{
		Stdin: string(reqPayload),
	}).Stdout(ctx)
	if err != nil {
		return nil, err
	}
	var out []any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		return nil, err
	}
	var versions []*ResourceVersion
	for _, o := range out {
		pl, err := json.Marshal(o)
		if err != nil {
			return nil, err
		}
		versions = append(versions, r.Version(dagger.JSON(pl)))
	}
	return versions, nil
}

// Get a specific version of the resource.
func (r *Resource) Version(version dagger.JSON) *ResourceVersion {
	return &ResourceVersion{
		Resource: r,
		Version:  version,
	}
}

// Fetch a version of the resource.
func (r *Resource) Get(
	ctx context.Context,
	// The version to fetch.
	version dagger.JSON,
	// Arbitrary parameters to pass to the resource.
	params dagger.JSON, // +optional
) (*dagger.Directory, error) {
	return r.Version(version).Get(ctx, params)
}

// Fetch a version of the resource.
func (r *Resource) LatestVersion(ctx context.Context) (*ResourceVersion, error) {
	vs, err := r.Check(ctx, "")
	if err != nil {
		return nil, err
	}
	if len(vs) == 0 {
		return nil, fmt.Errorf("resource %q: no versions found", r.Name)
	}
	return vs[len(vs)-1], nil
}

// Create or update a version of the resource.
func (r *Resource) Put(
	ctx context.Context,
	// Arbitrary parameters to pass to the resource.
	params dagger.JSON, // +optional
) (*ResourceVersion, error) {
	sourceJSON, err := r.Concourse.Interpolate(ctx, string(r.Source))
	if err != nil {
		return nil, fmt.Errorf("interpolate config vars: %w", err)
	}
	req := map[string]any{
		"source": json.RawMessage(sourceJSON),
	}
	if params != "" {
		req["params"] = json.RawMessage(params)
	}
	reqPayload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	stdout, err := r.Container.WithExec([]string{"/opt/resource/out"}, dagger.ContainerWithExecOpts{
		Stdin: string(reqPayload),
	}).Stdout(ctx)
	if err != nil {
		return nil, err
	}
	out := &ResourceVersion{
		Resource: r,
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// A version of a resource, with optional metadata.
type ResourceVersion struct {
	*Resource
	Version  dagger.JSON        `json:"version"`
	Metadata []ResourceMetadata `json:"metadata"`
}

// Fetch the resource version's content.
func (r *ResourceVersion) Get(
	ctx context.Context,
	// Arbitrary parameters to pass to the resource.
	params dagger.JSON, // +optional
) (*dagger.Directory, error) {
	sourceJSON, err := r.Concourse.Interpolate(ctx, string(r.Source))
	if err != nil {
		return nil, fmt.Errorf("interpolate config vars: %w", err)
	}
	req := map[string]any{
		"source":  json.RawMessage(sourceJSON),
		"version": json.RawMessage(r.Version),
	}
	if params != "" {
		paramsJSON, err := r.Concourse.Interpolate(ctx, string(params))
		if err != nil {
			return nil, fmt.Errorf("interpolate config vars: %w", err)
		}
		req["params"] = json.RawMessage(paramsJSON)
	}
	reqPayload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return r.Container.
			WithDirectory("/resource", dag.Directory()).
			WithExec([]string{"/opt/resource/in", "/resource"}, dagger.ContainerWithExecOpts{
				Stdin: string(reqPayload),
			}).
			Directory("/resource"),
		nil
}

type ResourceMetadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
