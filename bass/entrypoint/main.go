package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
)

func main() {
	ctx := context.Background()
	ctx = bass.WithTrace(ctx, &bass.Trace{})
	ctx = ioctx.StderrToContext(ctx, os.Stderr)

	slogOpts := &tint.Options{
		TimeFormat: time.TimeOnly,
		NoColor:    false,
		Level:      slog.LevelWarn,
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

	if parentName == "" {
		return initModule(ctx, modSrcDir, modName)
	}

	filePath := filepath.Join(modSrcDir, fmt.Sprintf("%s.bass", parentName))

	dir, base := filepath.Split(filePath)
	if dir == "" {
		dir = "."
	}

	cmd := bass.NewHostPath(
		dir,
		bass.ParseFileOrDirPath(filepath.ToSlash(base)),
	)

	thunk := bass.Thunk{
		Args: []bass.Value{cmd},
	}

	sess := bass.NewBass()

	mod, err := sess.Load(ctx, thunk)
	if err != nil {
		return nil, err
	}

	if fnName == "" {
		fnName = "new"
	}

	var fn bass.Applicative
	if err := mod.GetDecode(bass.Symbol(strcase.ToKebab(fnName)), &fn); err != nil {
		return nil, fmt.Errorf("failed to get new function: %w", err)
	}

	var argsList bass.List
	if fnName == "new" {
		argsList = bass.NewList(args)
	} else {
		argsList = bass.NewList(self, args)
	}

	ret, err := bass.Trampoline(ctx, fn.Call(ctx, argsList, mod, bass.Identity))
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

func initModule(ctx context.Context, modSrcDir, modName string) (_ any, rerr error) {
	dagMod := dag.Module()

	bassScripts, err := filepath.Glob(filepath.Join(modSrcDir, "*.bass"))
	if err != nil {
		return nil, fmt.Errorf("failed to list bass scripts: %w", err)
	}

	slog.Debug("found scripts", "scripts", bassScripts)

	sess := bass.NewBass()

	for _, filePath := range bassScripts {
		name, _ := strings.CutSuffix(filepath.Base(filePath), ".bass")

		objDef := dag.TypeDef().WithObject(name)

		dir, base := filepath.Split(filePath)
		if dir == "" {
			dir = "."
		}

		cmd := bass.NewHostPath(
			dir,
			bass.ParseFileOrDirPath(filepath.ToSlash(base)),
		)

		thunk := bass.Thunk{
			Args: []bass.Value{cmd},
		}

		bassMod, err := sess.Load(ctx, thunk)
		if err != nil {
			return nil, err
		}

		if strcase.ToCamel(name) == strcase.ToCamel(modName) {
			var desc string
			if err := bassMod.GetDecode("*description*", &desc); err == nil {
				dagMod = dagMod.WithDescription(desc)
			}
		}

		for name, v := range bassMod.Bindings {
			bassFnName := name.String()

			var ann bass.Annotated
			if err := bassMod.GetDecode(name, &ann); err != nil {
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
				return nil, fmt.Errorf("failed to get type of annotated value: %w", err)
			}
			funDef := dag.Function(bassFnName, retDef)

			// Wire up description
			var desc string
			if err := ann.Meta.GetDecode(bass.DocMetaBinding, &desc); err == nil {
				funDef = funDef.WithDescription(desc)
			}

			// Wire up arguments
			var app bass.Applicative
			if err := v.Decode(&app); err == nil {
				v = app.Unwrap()
			}
			var op *bass.Operative
			if err := v.Decode(&op); err != nil {
				slog.Info("not an operative; skipping", "error", err)
				continue
			}
			var args bass.List
			if err := op.Bindings.Decode(&args); err != nil {
				slog.Info("args is not a list; skipping", "error", err)
				continue
			}

			i := 0
			err = bass.Each(args, func(arg bass.Value) error {
				i++
				switch i {
				case 1:
					// skip 'self' arg
					return nil
				case 2:
					var binds bass.Bind
					if err := arg.Decode(&binds); err != nil {
						return fmt.Errorf("arg must be named bindings, {:like this} (%w)", err)
					}
					var argName string
					i := 0
					for _, b := range binds {
						if i%2 == 0 {
							var sym bass.Keyword
							if err := b.Decode(&sym); err != nil {
								return fmt.Errorf("arg must be named bindings, {:like this} (%w)", err)
							}
							argName = sym.Symbol().String()
						} else {
							var ann bass.Annotate
							if err := b.Decode(&ann); err != nil {
								return fmt.Errorf("arg must be named bindings, {:like this} (%w)", err)
							}
							metaVal, err := bass.Trampoline(ctx, ann.MetaBind().Eval(ctx, bass.NewEmptyScope(), bass.Identity))
							if err != nil {
								return fmt.Errorf("failed to eval binding metadata: %w", err)
							}
							var scope *bass.Scope
							if err := metaVal.Decode(&scope); err != nil {
								return fmt.Errorf("binding metadata must evaluate to a scope: %w", err)
							}
							var argType bass.Value
							if err := scope.GetDecode("type", &argType); err != nil {
								return fmt.Errorf("failed to get annotated type for %s.%s: %w", bassFnName, argName, err)
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
								return fmt.Errorf("failed to get type of annotated value: %w", err)
							}
							funDef = funDef.WithArg(argName, argDef, opts)
						}
						i++
					}
				default:
					return fmt.Errorf("%s: too many arguments", bassFnName)
				}

				// 					var ann bass.Annotated
				// 					if err := arg.Decode(&ann); err != nil {
				// 						return fmt.Errorf("failed to get annotated value of arg: %w", err)
				// 					}

				// 					var argType bass.Value
				// 					if err := ann.Meta.GetDecode("type", &argType); err != nil {
				// 						return fmt.Errorf("failed to get annotated type for %s.%s: %w", bassFnName, arg, err)
				// 					}
				// 					argDef, err := typeOf(argType)
				// 					if err != nil {
				// 						return fmt.Errorf("failed to get type of annotated value: %w", err)
				// 					}

				// 					// TODO description
				// 					funDef = funDef.WithArg(arg.String(), argDef)
				return nil
			})
			if err != nil {
				return nil, err
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
	default:
		return nil, fmt.Errorf("unsupported type: %T", val)
	}
}
