package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/concourse/concourse/atc"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// Build is a StepVisitor helper used for traversing a StepConfig and
// calling configured hooks on the "base" step types, i.e. step types that do
// not contain any other steps.
//
// Build must be updated with any new step type added. Steps which wrap
// other steps must recurse through them, while steps which are "base" steps
// must have a hook added for them, called when they visit the Build.
type Build struct {
	// Re-assigned throughout the visiting process without mutating.
	Ctx context.Context

	Concourse *Concourse
	Pipeline  *Pipeline

	// Runtime state modified as steps are executed.
	State *BuildState

	// // OnTask will be invoked for any *TaskStep present in the StepConfig.
	// OnTask func(*atc.TaskStep) error

	// // OnGet will be invoked for any *GetStep present in the StepConfig.
	// OnGet func(*atc.GetStep) error

	// // OnPut will be invoked for any *PutStep present in the StepConfig.
	// OnPut func(*atc.PutStep) error

	// // OnRun will be invoked for any *RunStep present in the StepConfig.
	// OnRun func(*atc.RunStep) error

	// // OnSetPipeline will be invoked for any *SetPipelineStep present in the StepConfig.
	// OnSetPipeline func(*atc.SetPipelineStep) error

	// // OnLoadVar will be invoked for any *LoadVarStep present in the StepConfig.
	// OnLoadVar func(*atc.LoadVarStep) error
}

type Path string

type BuildState struct {
	Assets map[string]*Directory

	l sync.Mutex
}

func (s *BuildState) Asset(name string) (*Directory, bool) {
	s.l.Lock()
	defer s.l.Unlock()
	if s.Assets == nil {
		return nil, false
	}
	dir, found := s.Assets[name]
	return dir, found
}

func (s *BuildState) StoreAsset(name string, dir *Directory) {
	s.l.Lock()
	defer s.l.Unlock()
	if s.Assets == nil {
		s.Assets = map[string]*Directory{}
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
func (build Build) VisitTask(step *atc.TaskStep) error {
	ctx, span := Tracer().Start(build.Ctx, "task: "+step.Name)
	defer span.End()
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

	var taskCtr *Container
	if taskCfg.ImageResource != nil {
		taskCtr, err = build.Pipeline.imageResource(ctx, taskCfg.ImageResource.Type, taskCfg.ImageResource.Source, taskCfg.ImageResource.Params)
		if err != nil {
			return build.Error(err)
		}
	} else if taskCfg.RootfsURI != "" {
		return build.Error(fmt.Errorf("rootfs uri not supported"))
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
	taskCtr = taskCtr.WithExec(args, ContainerWithExecOpts{
		// Concourse doesn't respect the entrypoint.
		SkipEntrypoint: true,
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
func (build Build) VisitGet(step *atc.GetStep) error {
	ctx, span := Tracer().Start(build.Ctx, "get: "+step.Name)
	defer span.End()
	build.Ctx = ctx
	resource := build.Pipeline.Resource(step.ResourceName())
	var version *ResourceVersion
	if step.Version != nil {
		if step.Version.Latest {
		} else if step.Version.Every {
			return build.Error(fmt.Errorf("version: every not supported"))
		} else if step.Version.Pinned != nil {
			versionJSON, err := json.Marshal(step.Version.Pinned)
			if err != nil {
				return build.Error(err)
			}
			version = resource.Version(JSON(versionJSON))
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
	dir, err := version.Get(ctx, JSON(paramsJSON))
	if err != nil {
		return build.Error(err)
	}
	build.State.StoreAsset(step.Name, dir)
	dir, err = dir.Sync(ctx)
	return err
}

// VisitPut calls the OnPut hook if configured.
func (build Build) VisitPut(step *atc.PutStep) error {
	ctx, span := Tracer().Start(build.Ctx, "put: "+step.Name)
	defer span.End()
	build.Ctx = ctx
	return nil
}

// VisitRun calls the OnRun hook if configured.
func (build Build) VisitRun(step *atc.RunStep) error {
	ctx, span := Tracer().Start(build.Ctx, "run: "+step.Message)
	defer span.End()
	build.Ctx = ctx
	return nil
}

// VisitSetPipeline calls the OnSetPipeline hook if configured.
func (build Build) VisitSetPipeline(step *atc.SetPipelineStep) error {
	ctx, span := Tracer().Start(build.Ctx, "pipeline: "+step.Name)
	defer span.End()
	build.Ctx = ctx
	return nil
}

// VisitLoadVar calls the OnLoadVar hook if configured.
func (build Build) VisitLoadVar(step *atc.LoadVarStep) error {
	ctx, span := Tracer().Start(build.Ctx, "load_var: "+step.Name)
	defer span.End()
	build.Ctx = ctx
	return nil
}

// VisitTry recurses through to the wrapped step.
func (build Build) VisitTry(step *atc.TryStep) error {
	ctx, span := Tracer().Start(build.Ctx, "try")
	defer span.End()
	build.Ctx = ctx
	return step.Step.Config.Visit(build)
}

// VisitDo recurses through to the wrapped steps.
func (build Build) VisitDo(step *atc.DoStep) error {
	ctx, span := Tracer().Start(build.Ctx, "do")
	defer span.End()
	build.Ctx = ctx

	for _, sub := range step.Steps {
		err := sub.Config.Visit(build)
		if err != nil {
			return err
		}
	}

	return nil
}

// VisitInParallel recurses through to the wrapped steps.
func (build Build) VisitInParallel(step *atc.InParallelStep) error {
	ctx, span := Tracer().Start(build.Ctx, "in_parallel")
	defer span.End()
	build.Ctx = ctx

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

// VisitAcross recurses through to the wrapped step.
func (build Build) VisitAcross(step *atc.AcrossStep) error {
	ctx, span := Tracer().Start(build.Ctx, "across")
	defer span.End()
	build.Ctx = ctx

	return step.Step.Visit(build)
}

// VisitTimeout recurses through to the wrapped step.
func (build Build) VisitTimeout(step *atc.TimeoutStep) error {
	ctx, span := Tracer().Start(build.Ctx, "timeout")
	defer span.End()
	build.Ctx = ctx

	return step.Step.Visit(build)
}

// VisitRetry recurses through to the wrapped step.
func (build Build) VisitRetry(step *atc.RetryStep) error {
	ctx, span := Tracer().Start(build.Ctx, "retry")
	defer span.End()
	build.Ctx = ctx

	return step.Step.Visit(build)
}

// VisitOnSuccess recurses through to the wrapped step and hook.
func (build Build) VisitOnSuccess(step *atc.OnSuccessStep) error {
	ctx, span := Tracer().Start(build.Ctx, "on_success")
	defer span.End()
	build.Ctx = ctx

	err := step.Step.Visit(build)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(build)
}

// VisitOnFailure recurses through to the wrapped step and hook.
func (build Build) VisitOnFailure(step *atc.OnFailureStep) error {
	ctx, span := Tracer().Start(build.Ctx, "on_failure")
	defer span.End()
	build.Ctx = ctx

	err := step.Step.Visit(build)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(build)
}

// VisitOnAbort recurses through to the wrapped step and hook.
func (build Build) VisitOnAbort(step *atc.OnAbortStep) error {
	ctx, span := Tracer().Start(build.Ctx, "on_abort")
	defer span.End()
	build.Ctx = ctx

	err := step.Step.Visit(build)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(build)
}

// VisitOnError recurses through to the wrapped step and hook.
func (build Build) VisitOnError(step *atc.OnErrorStep) error {
	ctx, span := Tracer().Start(build.Ctx, "on_error")
	defer span.End()
	build.Ctx = ctx

	err := step.Step.Visit(build)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(build)
}

// VisitEnsure recurses through to the wrapped step and hook.
func (build Build) VisitEnsure(step *atc.EnsureStep) error {
	ctx, span := Tracer().Start(build.Ctx, "ensure")
	defer span.End()
	build.Ctx = ctx

	err := step.Step.Visit(build)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(build)
}
