package main

import (
	"concourse/internal/dagger"
	"concourse/internal/telemetry"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/concourse/concourse/atc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

type Build struct {
	// Re-assigned throughout the visiting process without mutating.
	Ctx context.Context

	Concourse *Concourse

	Pipeline *Pipeline

	// Input versions resolved via the job's passed: constraints.
	Inputs map[string]*ResourceVersion

	// Runtime state modified as steps are executed.
	State *BuildState
}

type BuildState struct {
	Assets map[string]*dagger.Directory

	l sync.Mutex
}

func (s *BuildState) Asset(name string) (*dagger.Directory, bool) {
	s.l.Lock()
	defer s.l.Unlock()
	if s.Assets == nil {
		return nil, false
	}
	dir, found := s.Assets[name]
	return dir, found
}

func (s *BuildState) StoreAsset(name string, dir *dagger.Directory) {
	s.l.Lock()
	defer s.l.Unlock()
	if s.Assets == nil {
		s.Assets = map[string]*dagger.Directory{}
	}
	s.Assets[name] = dir
}

type BuildError struct {
	Path  string
	Error error
}

func (pl *Pipeline) build(ctx context.Context) Build {
	return Build{
		Concourse: pl.Concourse,
		Pipeline:  pl,
		Ctx:       ctx,
		State:     &BuildState{},
	}
}

// VisitTask calls the OnTask hook if configured.
func (build Build) VisitTask(step *atc.TaskStep) (rerr error) {
	ctx, span := Tracer().Start(build.Ctx, "task: "+step.Name)
	defer telemetry.End(span, func() error { return rerr })

	build.Ctx = ctx

	var err error

	var taskCfg atc.TaskConfig
	if step.ConfigPath != "" {
		inputName, subPath, ok := strings.Cut(step.ConfigPath, "/")
		if !ok {
			return build.Error(fmt.Errorf("invalid config path: %s", step.ConfigPath))
		}
		dir, found := build.State.Asset(inputName)
		if !found {
			return build.Error(fmt.Errorf("undefined asset: %s", inputName))
		}
		configYAML, err := dir.File(subPath).Contents(ctx)
		if err != nil {
			return build.Error(err)
		}
		taskCfg, err = atc.NewTaskConfig([]byte(configYAML))
		if err != nil {
			return build.Error(err)
		}
	} else if step.Config != nil {
		taskCfg = *step.Config
	}

	var taskCtr *dagger.Container
	if taskCfg.ImageResource != nil {
		taskCtr, err = build.Pipeline.imageResource(ctx, taskCfg.ImageResource.Type, taskCfg.ImageResource.Source, taskCfg.ImageResource.Params)
		if err != nil {
			return build.Error(err)
		}
	} else if step.ImageArtifactName != "" {
		dir := build.State.Assets[step.ImageArtifactName]
		taskCtr, err = build.Pipeline.fetchedImage(ctx, dir)
		if err != nil {
			return build.Error(err)
		}
	} else if taskCfg.RootfsURI != "" {
		return build.Error(fmt.Errorf("rootfs uri not supported"))
	} else {
		return build.Error(fmt.Errorf("no image specified"))
	}

	for _, input := range taskCfg.Inputs {
		if input.Path == "" {
			input.Path = input.Name
		}
		asset, found := build.State.Asset(input.Name)
		if !found {
			return build.Error(fmt.Errorf("undefined asset: %s", input.Name))
		}
		taskCtr = taskCtr.WithDirectory(input.Path, asset)
	}

	args := append([]string{taskCfg.Run.Path}, taskCfg.Run.Args...)
	// HACK: this won't run with a TTY, so disable stty
	taskCtr = taskCtr.WithFile("/usr/bin/stty", taskCtr.File("/bin/true"))
	taskCtr = taskCtr.WithExec(args, dagger.ContainerWithExecOpts{
		InsecureRootCapabilities:      step.Privileged,
		ExperimentalPrivilegedNesting: true,
	})

	_, err = taskCtr.Sync(ctx)

	return err
}

func (build Build) Error(err error) error {
	span := trace.SpanFromContext(build.Ctx)
	span.SetStatus(codes.Error, err.Error())
	return err
}

// VisitGet calls the OnGet hook if configured.
func (build Build) VisitGet(step *atc.GetStep) (rerr error) {
	ctx, span := Tracer().Start(build.Ctx, "get: "+step.Name)
	defer telemetry.End(span, func() error { return rerr })
	build.Ctx = ctx
	resource := build.Pipeline.Resource(step.ResourceName())
	version := build.Inputs[step.Name]
	if version == nil && step.Version != nil {
		if step.Version.Latest {
			// nothing to do
		} else if step.Version.Every {
			return build.Error(fmt.Errorf("version: every not supported"))
		} else if step.Version.Pinned != nil {
			versionJSON, err := json.Marshal(step.Version.Pinned)
			if err != nil {
				return build.Error(err)
			}
			version = resource.Version(dagger.JSON(versionJSON))
		}
	}
	if version == nil {
		var err error
		version, err = resource.LatestVersion(build.Ctx)
		if err != nil {
			return build.Error(err)
		}
	}
	paramsJSON, err := json.Marshal(step.Params)
	if err != nil {
		return build.Error(err)
	}
	dir, err := version.Get(ctx, dagger.JSON(paramsJSON))
	if err != nil {
		return build.Error(err)
	}
	build.State.StoreAsset(step.Name, dir)
	_, err = dir.Sync(ctx)
	return err
}

// VisitPut calls the OnPut hook if configured.
func (build Build) VisitPut(step *atc.PutStep) (rerr error) {
	ctx, span := Tracer().Start(build.Ctx, "put: "+step.Name)
	defer telemetry.End(span, func() error { return rerr })
	build.Ctx = ctx
	return nil
}

// VisitRun calls the OnRun hook if configured.
func (build Build) VisitRun(step *atc.RunStep) (rerr error) {
	ctx, span := Tracer().Start(build.Ctx, "run: "+step.Message)
	defer telemetry.End(span, func() error { return rerr })
	build.Ctx = ctx
	return nil
}

// VisitSetPipeline calls the OnSetPipeline hook if configured.
func (build Build) VisitSetPipeline(step *atc.SetPipelineStep) (rerr error) {
	ctx, span := Tracer().Start(build.Ctx, "pipeline: "+step.Name)
	defer telemetry.End(span, func() error { return rerr })
	build.Ctx = ctx
	return nil
}

// VisitLoadVar calls the OnLoadVar hook if configured.
func (build Build) VisitLoadVar(step *atc.LoadVarStep) (rerr error) {
	ctx, span := Tracer().Start(build.Ctx, "load_var: "+step.Name)
	defer telemetry.End(span, func() error { return rerr })
	build.Ctx = ctx
	return nil
}

func (build Build) VisitTry(step *atc.TryStep) (rerr error) {
	// not worth the nesting
	// ctx, span := Tracer().Start(build.Ctx, "try")
	// defer telemetry.End(span, func() error { return rerr })
	// build.Ctx = ctx
	if err := step.Step.Config.Visit(build); err != nil {
		trace.SpanFromContext(build.Ctx).
			AddEvent("try.error.suppressed", trace.WithAttributes(
				attribute.String("error", err.Error())))
	}
	return nil
}

func (build Build) VisitDo(step *atc.DoStep) (rerr error) {
	// not worth the nesting
	// ctx, span := Tracer().Start(build.Ctx, "do")
	// defer telemetry.End(span, func() error { return rerr })
	// build.Ctx = ctx

	for _, sub := range step.Steps {
		err := sub.Config.Visit(build)
		if err != nil {
			return err
		}
	}

	return nil
}

func (build Build) VisitInParallel(step *atc.InParallelStep) (rerr error) {
	// not worth the noise, the spans already show that they're parallel
	// ctx, span := Tracer().Start(build.Ctx, "in_parallel")
	// defer telemetry.End(span, func() error { return rerr })
	// build.Ctx = ctx

	subBuild := build

	var eg *errgroup.Group
	if step.Config.FailFast {
		eg, subBuild.Ctx = errgroup.WithContext(build.Ctx)
	} else {
		eg = new(errgroup.Group)
	}
	for _, sub := range step.Config.Steps {
		sub := sub
		eg.Go(func() error {
			return sub.Config.Visit(subBuild)
		})
	}

	return eg.Wait()
}

func (build Build) VisitAcross(step *atc.AcrossStep) (rerr error) {
	ctx, span := Tracer().Start(build.Ctx, "across")
	defer telemetry.End(span, func() error { return rerr })
	build.Ctx = ctx

	return step.Step.Visit(build)
}

func (build Build) VisitTimeout(step *atc.TimeoutStep) (rerr error) {
	ctx, span := Tracer().Start(build.Ctx, "timeout")
	defer telemetry.End(span, func() error { return rerr })
	build.Ctx = ctx

	// TODO
	return step.Step.Visit(build)
}

func (build Build) VisitRetry(step *atc.RetryStep) (rerr error) {
	ctx, span := Tracer().Start(build.Ctx, "retry")
	defer telemetry.End(span, func() error { return rerr })
	build.Ctx = ctx

	// TODO
	return step.Step.Visit(build)
}

func (build Build) VisitOnSuccess(step *atc.OnSuccessStep) error {
	err := step.Step.Visit(build)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(build)
}

func (build Build) VisitOnFailure(step *atc.OnFailureStep) (rerr error) {
	// ctx, span := Tracer().Start(build.Ctx, "on_failure")
	// defer telemetry.End(span, func() error { return rerr })
	// build.Ctx = ctx

	err := step.Step.Visit(build)
	if err != nil {
		ctx, span := Tracer().Start(build.Ctx, "on_failure")
		defer telemetry.End(span, func() error { return rerr })
		build.Ctx = ctx
		return errors.Join(err, step.Hook.Config.Visit(build))
	}

	return nil
}

func (build Build) VisitOnAbort(step *atc.OnAbortStep) (rerr error) {
	err := step.Step.Visit(build)

	if build.Ctx.Err() != nil {
		ctx, span := Tracer().Start(build.Ctx, "on_abort")
		defer telemetry.End(span, func() error { return rerr })
		build.Ctx = ctx
		return errors.Join(err, step.Hook.Config.Visit(build))
	}

	return err
}

func (build Build) VisitOnError(step *atc.OnErrorStep) (rerr error) {
	err := step.Step.Visit(build)
	if err != nil {
		ctx, span := Tracer().Start(build.Ctx, "on_error")
		defer telemetry.End(span, func() error { return rerr })
		build.Ctx = ctx
		// TODO no distinction from failure?
		return step.Hook.Config.Visit(build)
	}

	return nil
}

func (build Build) VisitEnsure(step *atc.EnsureStep) (rerr error) {
	defer func() {
		ctx, span := Tracer().Start(build.Ctx, "ensure")
		defer telemetry.End(span, func() error { return rerr })
		build.Ctx = ctx
		rerr = errors.Join(rerr, step.Hook.Config.Visit(build))
	}()
	return step.Step.Visit(build)
}
