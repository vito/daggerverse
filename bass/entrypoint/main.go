package main

import (
	"context"
	"embed"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
	"github.com/iancoleman/strcase"
	"github.com/lmittmann/tint"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/bass/pkg/zapctx"
	"go.uber.org/zap/zapcore"
)

func main() {
	ctx := context.Background()
	ctx = bass.WithTrace(ctx, &bass.Trace{})
	ctx = ioctx.StderrToContext(ctx, os.Stderr)
	ctx = zapctx.ToContext(ctx, bass.StdLogger(zapcore.DebugLevel))

	slogOpts := &tint.Options{
		TimeFormat: time.TimeOnly,
		NoColor:    false,
		Level:      slog.LevelInfo,
	}
	slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, slogOpts)))

	ctx, runs := bass.TrackRuns(ctx)
	defer func() { _ = runs.StopAndWait() }()

	fnCall := dag.CurrentFunctionCall()
	parentName, err := fnCall.ParentName(ctx)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
	fnName, err := fnCall.Name(ctx)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
	parentJson, err := fnCall.Parent(ctx)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
	fnArgs, err := fnCall.InputArgs(ctx)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}

	inputArgs := map[string][]byte{}
	for _, fnArg := range fnArgs {
		argName, err := fnArg.Name(ctx)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(2)
		}
		argValue, err := fnArg.Value(ctx)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(2)
		}
		inputArgs[argName] = []byte(argValue)
	}

	slog.Debug("invoking", "parentName", parentName, "fnName", fnName, "inputArgs", inputArgs, "parentJson", parentJson)

	modSrcDir := os.Args[1]
	modName := os.Args[2]

	result, err := invoke(ctx, modSrcDir, modName, []byte(parentJson), parentName, fnName, inputArgs)
	if err != nil {
		cli.WriteError(ctx, err)
		os.Exit(2)
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}

	slog.Debug("returning", "result", string(resultBytes))

	_, err = fnCall.ReturnValue(ctx, dagger.JSON(resultBytes))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
}

//go:embed init.bass
var initSrc embed.FS

func invoke(ctx context.Context, modSrcDir string, modName string, parentJSON []byte, parentName string, fnName string, inputArgs map[string][]byte) (_ any, err error) {
	pool, err := runtimes.NewPool(ctx, &bass.Config{
		Runtimes: []bass.RuntimeConfig{
			{
				Platform: bass.LinuxPlatform,
				Runtime:  runtimes.DaggerName,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime pool: %w", err)
	}

	ctx = bass.WithRuntimePool(ctx, pool)

	var self bass.Value
	if err := bass.UnmarshalJSON(parentJSON, &self); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parent object: %w", err)
	}

	args := bass.NewEmptyScope()
	for k, v := range inputArgs {
		var val bass.Value
		if err := bass.UnmarshalJSON(v, &val); err != nil {
			return nil, fmt.Errorf("failed to unmarshal input arg %s: %w", k, err)
		}
		args.Set(bass.Symbol(k), val)
	}

	cmd := bass.NewHostPath(
		modSrcDir,
		bass.ParseFileOrDirPath(modName+".bass"),
	)

	thunk := bass.Thunk{
		Args: []bass.Value{cmd},
	}

	sess, err := initBass(ctx)
	if err != nil {
		return nil, err
	}

	bassMod, err := sess.Load(ctx, thunk)
	if err != nil {
		return nil, err
	}

	if parentName == "" {
		return initModule(ctx, bassMod)
	}

	if fnName == "" {
		fnName = "new"
	}

	var objMod *bass.Scope
	if err := bassMod.GetDecode(bass.Symbol(strcase.ToCamel(parentName)), &objMod); err != nil {
		return nil, fmt.Errorf("failed to get parent object: %w", err)
	}

	var fn bass.Applicative
	if err := objMod.GetDecode(bass.Symbol(strcase.ToKebab(fnName)), &fn); err != nil {
		return nil, fmt.Errorf("failed to get new function: %w", err)
	}

	var argsList bass.List
	if fnName == "new" {
		argsList = bass.NewList(args)
	} else {
		argsList = bass.NewList(self, args)
	}

	ret, err := bass.Trampoline(ctx, fn.Call(ctx, argsList, bassMod, bass.Identity))
	if err != nil {
		return nil, fmt.Errorf("failed to call function: %w", err)
	}

	var thnk bass.Thunk
	if err := ret.Decode(&thnk); err == nil {
		return runtimes.NewDagger().Container(ctx, thnk, false)
	}

	var path bass.ThunkPath
	if err := ret.Decode(&path); err == nil {
		baseCtr, err := runtimes.NewDagger().Container(ctx, path.Thunk, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create container: %w", err)
		}
		fsp := path.Path.FilesystemPath()
		if fsp.IsDir() {
			return baseCtr.Directory(fsp.String()), nil
		}
		return baseCtr.File(fsp.String()), nil
	}

	return ret, nil
}

func initBass(ctx context.Context) (*bass.Session, error) {
	scope := bass.NewStandardScope()
	if err := initPlatform(ctx, scope); err != nil {
		return nil, fmt.Errorf("failed to init platform vars: %w", err)
	}
	initPath := bass.NewFSPath(initSrc, bass.ParseFileOrDirPath("init.bass"))
	if _, err := bass.EvalFSFile(ctx, scope, initPath); err != nil {
		return nil, fmt.Errorf("failed to eval init.bass: %w", err)
	}
	return bass.NewSession(scope), nil
}

func initPlatform(ctx context.Context, scope *bass.Scope) error {
	// Set the default OCI platform as *platform*.
	platStr, err := dag.DefaultPlatform(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default platform: %w", err)
	}
	scope.Set("*platform*", bass.String(platStr))

	// Set the non-OS portion of the OCI platform as *arch* so that we include v7
	// in arm/v7.
	_, arch, _ := strings.Cut(string(platStr), "/")
	scope.Set("*arch*", bass.String(arch))

	return nil
}

func initModule(ctx context.Context, bassMod *bass.Scope) (_ any, rerr error) {
	dagMod := dag.Module()

	var desc string
	if err := bassMod.GetDecode("*description*", &desc); err == nil {
		dagMod = dagMod.WithDescription(desc)
	}

	for name, val := range bassMod.Bindings {
		var ann bass.Annotated
		if err := val.Decode(&ann); err != nil {
			slog.Info("not annotated; assuming internal", "name", name, "error", err)
			continue
		}

		var objName bass.Symbol
		if err := ann.Meta.GetDecode("object", &objName); err != nil {
			slog.Info("no return type defined; assuming internal", "name", name, "error", err)
			continue
		}
		var objOpts dagger.TypeDefWithObjectOpts
		var desc string
		if err := ann.Meta.GetDecode(bass.DocMetaBinding, &desc); err == nil {
			objOpts.Description = desc
		}

		objDef := dag.TypeDef().WithObject(objName.String(), objOpts)

		var objScope *bass.Scope
		if err := val.Decode(&objScope); err != nil {
			slog.Info("not a scope; skipping", "name", name, "error", err)
			continue
		}
		for subName, subVal := range objScope.Bindings {
			bassFnName := subName.String()

			var ann bass.Annotated
			if err := subVal.Decode(&ann); err != nil {
				slog.Info("not annotated; assuming internal", "name", bassFnName, "error", err)
				continue
			}

			// Wire up return type
			var retType bass.Value
			if err := ann.Meta.GetDecode("type", &retType); err != nil {
				slog.Info("no return type defined; assuming internal", "name", bassFnName, "error", err)
				continue
			}
			retDef, err := typeOf(retType)
			if err != nil {
				return nil, fmt.Errorf("failed to get type of return value: %w", err)
			}
			funDef := dag.Function(bassFnName, retDef)

			// Wire up args
			var args bass.Scope
			if err := ann.Meta.GetDecode("args", &args); err != nil {
				slog.Info("no return type defined; assuming internal", "name", bassFnName, "error", err)
				continue
			}
			err = args.Each(func(argName bass.Symbol, config bass.Value) error {
				var scope *bass.Scope
				if err := config.Decode(&scope); err != nil {
					return fmt.Errorf("binding metadata must evaluate to a scope: %w", err)
				}
				var argType bass.Value
				if err := scope.GetDecode("type", &argType); err != nil {
					return fmt.Errorf("failed to get annotated type for %s.%s from %s: %w", bassFnName, argName, scope, err)
				}
				var opts dagger.FunctionWithArgOpts
				var defaultVal bass.Value
				if err := scope.GetDecode("default", &defaultVal); err == nil {
					payload, err := bass.MarshalJSON(defaultVal)
					if err != nil {
						return fmt.Errorf("failed to marshal default value: %w", err)
					}
					opts.DefaultValue = dagger.JSON(payload)
				}
				var desc string
				if err := scope.GetDecode(bass.DocMetaBinding, &desc); err == nil {
					opts.Description = desc
				}
				argDef, err := typeOf(argType)
				if err != nil {
					return fmt.Errorf("failed to get type of arg value: %w", err)
				}
				funDef = funDef.WithArg(argName.String(), argDef, opts)
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to wire up args: %w", err)
			}

			// Wire up description
			var desc string
			if err := ann.Meta.GetDecode(bass.DocMetaBinding, &desc); err == nil {
				funDef = funDef.WithDescription(desc)
			}

			if bassFnName == "new" {
				objDef = objDef.WithConstructor(funDef)
			} else {
				objDef = objDef.WithFunction(funDef)
			}
		}

		dagMod = dagMod.WithObject(objDef)
	}

	return dagMod, nil
}

func typeOf(val bass.Value) (*dagger.TypeDef, error) {
	def := dag.TypeDef()
	switch x := val.(type) {
	case bass.List:
		elemType, err := typeOf(x.First())
		if err != nil {
			return nil, err
		}
		return def.WithListOf(elemType), nil
	case bass.Symbol:
		switch x {
		case "String":
			return def.WithKind(dagger.StringKind), nil
		case "Integer":
			return def.WithKind(dagger.IntegerKind), nil
		case "Boolean":
			return def.WithKind(dagger.BooleanKind), nil
		case "Void":
			return def.WithKind(dagger.VoidKind), nil
		default:
			return def.WithObject(x.String()), nil
		}
	case bass.Annotate:
		return typeOf(x.Value)
	default:
		return nil, fmt.Errorf("unsupported type: %T", val)
	}
}
