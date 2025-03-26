package main

import (
	"context"
	"dagger/workspace/internal/dagger"
	"dagger/workspace/internal/telemetry"
	_ "embed"
	"fmt"
	"strings"
	"sync"
)

type Workspace struct {
	// +private
	Model string

	// +private
	Evals int

	// The authoritative documentation.
	README string

	// The current system prompt.
	SystemPrompt string
}

//go:embed README.md
var README string

//go:embed INITIAL.md
var INITIAL string

func New(
	// +default=""
	model string,
	// +default=2
	evals int,
) *Workspace {
	return &Workspace{
		Model:  model,
		Evals:  evals,
		README: README,
		// SystemPrompt: INITIAL,
		// SystemPrompt: README,
	}
}

// Set the system prompt for future evaluations.
func (w *Workspace) WithSystemPrompt(prompt string) *Workspace {
	w.SystemPrompt = prompt
	return w
}

// Evaluate the LLM and return the history of prompts, responses, and tool calls.
func (w *Workspace) Evaluate(ctx context.Context) (string, error) {
	reports := make([]string, w.Evals)
	wg := new(sync.WaitGroup)
	var successCount int
	for attempt := range w.Evals {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx, span := Tracer().Start(ctx, fmt.Sprintf("attempt %d", attempt+1),
				telemetry.Reveal())

			var rerr error
			defer telemetry.End(span, func() error { return rerr })

			report := new(strings.Builder)
			defer func() { reports[attempt] = report.String() }()

			fmt.Fprintf(report, "## Attempt %d\n", attempt+1)
			fmt.Fprintln(report)

			eval := w.evaluate(attempt)

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
			}
		}()
	}

	wg.Wait()

	finalReport := new(strings.Builder)
	fmt.Fprintln(finalReport, "# Model:", w.Model)
	fmt.Fprintln(finalReport)
	fmt.Fprintln(finalReport, "## All Attempts")
	fmt.Fprintln(finalReport)
	for _, report := range reports {
		fmt.Fprint(finalReport, report)
	}

	fmt.Fprintln(finalReport, "## Final Report")
	fmt.Fprintln(finalReport)
	fmt.Fprintf(finalReport, "SUCCESS RATE: %d/%d (%.f%%)\n", successCount, w.Evals, float64(successCount)/float64(w.Evals)*100)

	return finalReport.String(), nil
}

func (w *Workspace) EvaluateAllModelsOnce(ctx context.Context) ([]string, error) {
	models := []string{
		"gpt-4o",
		"gemini-2.0-flash",
		"claude-3-5-sonnet-latest",
		"claude-3-7-sonnet-latest",
	}
	reports := make([]string, len(models))
	wg := new(sync.WaitGroup)
	for i, model := range models {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, span := Tracer().Start(ctx, fmt.Sprintf("model: %s", model),
				telemetry.Reveal())
			report, err := New(model, 1).Evaluate(ctx)
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

func (w *Workspace) evaluate(attempt int) *dagger.EvalsReport {
	return dag.Evals().
		WithAttempt(attempt + 1).
		WithModel(w.Model).
		WithSystemPrompt(w.SystemPrompt).
		BuildMulti()
}
