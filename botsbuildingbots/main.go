package main

import (
	"context"
	"dagger/botsbuildingbots/internal/dagger"
	_ "embed"
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
	return dag.LLM(dagger.LLMOpts{Model: m.WriterModel}).
		WithWorkspace(dag.Workspace(dagger.WorkspaceOpts{
			Model:        m.EvalModel,
			Attempts:     m.Attempts,
			SystemPrompt: m.InitialPrompt,
		}))
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
		SetFile("README", m.Scheme).
		WithPrompt(`Read the README and generate the best system prompt for it. Keep going until all attempts succeed.`).
		Workspace().
		SystemPrompt(ctx)
}

func (m *BotsBuildingBots) Explore(ctx context.Context) *dagger.Directory {
	return m.llm().
		WithPrompt(`You are a quality assurance engineer running a suite of LLM evals and finding any issues that various models have interpreting them.`).
		WithPrompt(`Focus on exploration. Find evals that work on some models, but not others.`).
		WithPrompt(`If an eval fails for all models, don't bother running it again, but if there is partial success, try running it again or with different models.`).
		WithPrompt(`BEWARE: you will almost certainly hit rate limits. Find something else to do with another model in that case, or back off for a bit.`).
		WithPrompt(`Keep performing evaluations against various models. Take notes and stop once you become exhausted, returning the notes directory.`).
		SetDirectory("notes", dag.Directory()).
		Directory()
}
