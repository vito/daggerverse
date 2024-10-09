package main

import (
	"concourse/internal/telemetry"
	"context"
	"encoding/json"
	"fmt"

	"github.com/concourse/concourse/atc"
	"golang.org/x/sync/errgroup"
)

type Pipeline struct {
	Concourse     *Concourse
	ResourceTypes []ResourceType
	Resources     []Resource
	Jobs          []Job // +private
}

func (pl *Pipeline) Run(ctx context.Context) error {
	resourceStreams := make(map[Keyword]*PubSub[*ResourceVersion])
	for _, resource := range pl.Resources {
		resourceStreams[resource.Name] = NewBroadcast[*ResourceVersion]()
	}

	jobStreams := make(map[Keyword]*PubSub[Object[*ResourceVersion]])
	for _, job := range pl.Jobs {
		jobStreams[job.Name] = NewBroadcast[Object[*ResourceVersion]]()
	}

	eg, ctx := errgroup.WithContext(ctx)

	for _, job := range pl.Jobs {
		var cfg atc.JobConfig
		if err := json.Unmarshal([]byte(job.Config), &cfg); err != nil {
			return err
		}

		independentInputs := map[Keyword]Stream[*ResourceVersion]{}
		dependentInputs := []Stream[Object[*ResourceVersion]]{}
		for _, input := range cfg.Inputs() {
			if len(input.Passed) == 0 {
				independentInputs[input.Name] = resourceStreams[input.Resource].Subscribe()
			} else {
				for _, passed := range input.Passed {
					dependentInputs = append(dependentInputs, jobStreams[passed].Subscribe())
				}
			}
		}

		var jobInputStream Stream[Object[*ResourceVersion]]
		if len(dependentInputs) == 0 {
			jobInputStream = Aggregate(ctx, job.Name, independentInputs)
		} else if len(independentInputs) == 0 {
			jobInputStream = Intersect(ctx, job.Name, dependentInputs...)
		} else {
			jobInputStream = Intersect(ctx, job.Name, append(dependentInputs, Aggregate(ctx, job.Name, independentInputs))...)
		}

		successful := jobStreams[job.Name]

		job := job
		eg.Go(func() error {
			for {
				inputs, err := jobInputStream.Next(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					log.Println("failed to get inputs", "job", job.Name, "error", err)
					return err
				}
				buildCtx, buildSpan := Tracer().Start(ctx, "build: "+job.Name)
				stdio := telemetry.SpanStdio(buildCtx, "concourse")
				for name, input := range inputs {
					fmt.Fprintln(stdio.Stdout, "input:", name, "=>", input.Version)
				}
				build := pl.build(buildCtx)
				build.Inputs = inputs
				step := cfg.StepConfig()
				buildErr := step.Visit(build)
				telemetry.End(buildSpan, func() error { return buildErr })
				if buildErr == nil {
					successful.Emit(ctx, inputs)
				}
			}
		})
	}

	for _, resource := range pl.Resources {
		foundVersions := resourceStreams[resource.Name]
		allVersions := pl.Resource(resource.Name).infiniteStream("")

		eg.Go(func() error {
			for {
				version, err := allVersions.Next(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return err
				}
				foundVersions.Emit(ctx, version)
			}
		})
	}

	return eg.Wait()
}
