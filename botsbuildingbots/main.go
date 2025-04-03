package main

import (
	"context"
	"dagger/botsbuildingbots/internal/dagger"
	_ "embed"
	"fmt"
)

type BotsBuildingBots struct {
	Scheme        *dagger.File
	InitialPrompt string
	WriterModel   string
	EvalModel     string
	Attempts      int
}

func New(
	// The documentation for the tool calling scheme to generate a prompt for.
	// +defaultPath=./md/dagger_scheme.md
	scheme *dagger.File,
	// An initial system prompt to evaluate and use as a starting point.
	// +optional
	initialPrompt string,
	// Model to use to generate the prompt.
	// +optional
	model string,
	// Model to use to run the evaluations.
	// +optional
	evalModel string,
	// Number of evaluations to run in parallel.
	// +default=1
	attempts int,
) *BotsBuildingBots {
	return &BotsBuildingBots{
		Scheme:        scheme,
		InitialPrompt: initialPrompt,
		WriterModel:   model,
		EvalModel:     evalModel,
		Attempts:      attempts,
	}
}

func (m *BotsBuildingBots) llm() *dagger.LLM {
	return dag.LLM(dagger.LLMOpts{Model: m.WriterModel})
}

func (m *BotsBuildingBots) env() *dagger.Env {
	return dag.Env().
		WithWorkspaceInput("work",
			dag.Workspace(dagger.WorkspaceOpts{
				Attempts:     m.Attempts,
				SystemPrompt: m.InitialPrompt,
			}),
			"A space for you to work in.")
}

func (m *BotsBuildingBots) SystemPrompt(ctx context.Context) (string, error) {
	return m.llm().
		WithSystemPrompt(`You are an autonomous system prompt refinement loop.

Your job is to:
1. Generate a system prompt. START WITH ONE SENTENCE. Framing is PARAMOUNT.
2. Run the evaluations and analyze the results.
3. Generate a report summarizing:
	- Your current understanding of the failures or successes
  - Your analysis of the success rate and token usage cost
4. If improvement is needed, generate a new system prompt and repeat the cycle.
5. If the evaluation passes fully, output the final system prompt and stop.

You control this loop end-to-end. Do not treat this as a one-shot task. Continue refining until success is achieved.
`).
		WithEnv(m.env().
			WithFileInput("README", m.Scheme, "The README to consult for generating your system prompt.").
			WithWorkspaceOutput("work", "The workspace with the system prompt assigned."),
		).
		WithPrompt(`Read the README and generate the best system prompt for it. Keep going until all attempts succeed.`).
		Env().
		Output("work").
		AsWorkspace().
		SystemPrompt(ctx)
}

func (m *BotsBuildingBots) Explore(ctx context.Context) ([]string, error) {
	return m.llm().
		WithEnv(m.env().
			WithWorkspaceOutput("findings", "The workspace with all of your findings recorded.")).
		WithPrompt(`You are a quality assurance engineer running a suite of LLM evals and finding any issues that various models have interpreting them.`).
		WithPrompt(`Focus on exploration. Find evals that work on some models, but not others.`).
		WithPrompt(`If an eval fails for all models, don't bother running it again, but if there is partial success, try running it again or with different models.`).
		WithPrompt(`BEWARE: you will almost certainly hit rate limits. Find something else to do with another model in that case, or back off for a bit.`).
		WithPrompt(`Keep performing evaluations against various models, and record any interesting findings.`).
		Env().
		Output("findings").
		AsWorkspace().
		Findings(ctx)
}

func (m *BotsBuildingBots) Evaluate(ctx context.Context, model string, name string) ([]string, error) {
	return m.llm().
		WithEnv(m.env().
			WithWorkspaceOutput("findings", "The workspace with all of your findings recorded."),
		).
		WithPrompt(`You are a QA engineer running an LLM eval against a model`).
		WithPrompt(
			fmt.Sprintf(`Run the %q eval against the %q model and analyze the results.`,
				name, model,
			),
		).
		Env().
		Output("findings").
		AsWorkspace().
		Findings(ctx)
}
