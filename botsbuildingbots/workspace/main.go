package main

import (
	"context"
	"dagger/workspace/internal/dagger"
	"dagger/workspace/internal/telemetry"
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Workspace struct {
	// +private
	Model string

	// +private
	Attempts int

	// The current system prompt.
	SystemPrompt string

	// Observations made throughout running evaluations.
	Findings []string
}

var knownModels = []string{
	"gpt-4o",
	"gemini-2.0-flash",
	"claude-3-5-haiku-latest",
	"claude-3-5-sonnet-latest",
	"claude-3-7-sonnet-latest",
}

type EvalFunc = func(*dagger.Evals) *dagger.EvalsReport

var evals = map[string]EvalFunc{
	"BuildMulti":            (*dagger.Evals).BuildMulti,
	"BuildMultiNoVar":       (*dagger.Evals).BuildMultiNoVar,
	"ReadImplicitVars":      (*dagger.Evals).ReadImplicitVars,
	"SingleState":           (*dagger.Evals).SingleState,
	"SingleStateTransition": (*dagger.Evals).SingleStateTransition,
	"UndoSingle":            (*dagger.Evals).UndoSingle,
}

func New(
	// +default=2
	attempts int,
	// +default=""
	systemPrompt string,
) *Workspace {
	return &Workspace{
		Attempts:     attempts,
		SystemPrompt: systemPrompt,
	}
}

// Set the system prompt for future evaluations.
func (w *Workspace) WithSystemPrompt(prompt string) *Workspace {
	w.SystemPrompt = prompt
	return w
}

// Backoff sleeps for the given duration in seconds.
//
// Use this if you're getting rate limited and have nothing better to do.
func (w *Workspace) Backoff(seconds int) *Workspace {
	time.Sleep(time.Duration(seconds) * time.Second)
	return w
}

// The list of possible evals you can run.
func (w *Workspace) EvalNames() []string {
	var names []string
	for eval := range evals {
		names = append(names, eval)
	}
	sort.Strings(names)
	return names
}

// The list of models that you can run evaluations against.
func (w *Workspace) KnownModels() []string {
	return knownModels
}

// Record an interesting finding after performing evaluations.
func (w *Workspace) RecordFinding(finding string) *Workspace {
	w.Findings = append(w.Findings, finding)
	return w
}

// Run an evaluation and return its report.
func (w *Workspace) Evaluate(
	ctx context.Context,
	// The evaluation to run.
	eval string,
	// The model to evaluate.
	// +default=""
	model string,
) (_ string, rerr error) {
	evalFn, ok := evals[eval]
	if !ok {
		return "", fmt.Errorf("unknown evaluation: %s", eval)
	}

	reports := make([]string, w.Attempts)
	wg := new(sync.WaitGroup)
	var successCount int
	for attempt := range w.Attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()

			report := new(strings.Builder)

			var rerr error
			ctx, span := Tracer().Start(ctx, fmt.Sprintf("%s: attempt %d", eval, attempt+1),
				telemetry.Reveal())
			defer telemetry.End(span, func() error { return rerr })
			stdio := telemetry.SpanStdio(ctx, "")
			defer stdio.Close()

			defer func() {
				reports[attempt] = report.String()
				fmt.Fprint(stdio.Stdout, report.String())
			}()

			fmt.Fprintf(report, "## Attempt %d\n", attempt+1)
			fmt.Fprintln(report)

			eval := w.evaluate(model, attempt, evalFn)

			evalReport, err := eval.Report(ctx)
			if err != nil {
				rerr = err
				return
			}
			fmt.Fprintln(report, evalReport)

			succeeded, err := eval.Succeeded(ctx)
			if err != nil {
				rerr = err
				return
			}
			if succeeded {
				successCount++
			} else {
				rerr = errors.New("evaluation failed")
			}
		}()
	}

	wg.Wait()

	finalReport := new(strings.Builder)
	fmt.Fprintln(finalReport, "# Model:", model)
	fmt.Fprintln(finalReport)
	fmt.Fprintln(finalReport, "## All Attempts")
	fmt.Fprintln(finalReport)
	for _, report := range reports {
		fmt.Fprint(finalReport, report)
	}

	fmt.Fprintln(finalReport, "## Final Report")
	fmt.Fprintln(finalReport)
	fmt.Fprintf(finalReport, "SUCCESS RATE: %d/%d (%.f%%)\n", successCount, w.Attempts, float64(successCount)/float64(w.Attempts)*100)

	return finalReport.String(), nil
}

// Run an evaluation across all known models in parallel.
func (w *Workspace) evaluateAcrossModels(
	ctx context.Context,
	eval string,
	models []string,
) ([]string, error) {
	reports := make([]string, len(models))
	wg := new(sync.WaitGroup)
	for i, model := range models {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, span := Tracer().Start(ctx, fmt.Sprintf("model: %s", model),
				telemetry.Reveal())
			report, err := New(w.Attempts, w.SystemPrompt).Evaluate(ctx, eval, model)
			telemetry.End(span, func() error { return err })
			if err != nil {
				reports[i] = fmt.Sprintf("ERROR: %s", err)
			} else {
				reports[i] = report
			}
		}()
	}
	wg.Wait()
	return reports, nil
}

func (w *Workspace) evaluate(model string, attempt int, evalFn EvalFunc) *dagger.EvalsReport {
	return evalFn(
		dag.Evals().
			WithModel(model).
			WithAttempt(attempt + 1).
			WithSystemPrompt(w.SystemPrompt),
	)
}
