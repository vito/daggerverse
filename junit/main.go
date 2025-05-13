// A generated module for Junit functions
//
// This module has been generated via dagger init and serves as a reference to
// basic module structure as you get started with Dagger.
//
// Two functions have been pre-created. You can modify, delete, or add to them,
// as needed. They demonstrate usage of arguments and return types using simple
// echo and grep commands. The functions can be called from the dagger CLI or
// from one of the SDKs.
//
// The first line in this comment block is a short description line and the
// rest is a long description with more detail on the module's purpose or usage,
// if appropriate. All modules should have a short description.

package main

import (
	"context"
	"dagger/junit/internal/dagger"
	"dagger/junit/internal/telemetry"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joshdk/go-junit"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

type Junit struct{}

// Report converts a JUnit XML report to OpenTelemetry spans.
func (m *Junit) ReportAll(ctx context.Context, reports *dagger.Directory) error {
	paths, err := reports.Glob(ctx, "**/*.xml")
	if err != nil {
		return err
	}

	eg := new(errgroup.Group)
	for _, path := range paths {
		report := reports.File(path)
		eg.Go(func() error {
			return m.Report(ctx, report)
		})
	}

	return eg.Wait()
}

// Report converts a JUnit XML report to OpenTelemetry spans.
func (m *Junit) Report(ctx context.Context, report *dagger.File) error {
	reportXML, err := report.Contents(ctx)
	if err != nil {
		return err
	}

	// Parse the JUnit XML
	suites, err := junit.IngestReader(strings.NewReader(reportXML))
	if err != nil {
		return err
	}

	fmt.Println("\nParsed JUnit Report Details:")
	fmt.Println("============================")

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	now := time.Now()

	for _, suite := range suites {
		enc.Encode(suite)

		// unfortunately JUnit doesn't record suite/test start time, so we'll work
		// backwards from the longest test duration and pretend that's when we
		// started
		suiteStartTime := now
		for _, test := range suite.Tests {
			testStartTime := now.Add(-test.Duration)
			if testStartTime.Before(suiteStartTime) {
				testStartTime = suiteStartTime
			}
		}

		suiteCtx, suiteSpan := Tracer().Start(ctx, suite.Name,
			trace.WithTimestamp(suiteStartTime),
			trace.WithAttributes(
				attribute.String("junit.suite.package", suite.Package),
			),
			telemetry.Reveal(),
		)

		for _, test := range suite.Tests {
			testStartTime := now.Add(-test.Duration)

			testCtx, testSpan := Tracer().Start(suiteCtx, test.Name, trace.WithTimestamp(testStartTime))

			stdio := telemetry.SpanStdio(testCtx, "")

			if test.Message != "" {
				fmt.Fprintln(stdio.Stdout, test.Message)
			}

			if test.SystemOut != "" {
				fmt.Fprint(stdio.Stdout, test.SystemOut)
			}
			if test.SystemErr != "" {
				fmt.Fprint(stdio.Stderr, test.SystemErr)
			}

			switch test.Status {
			case "passed":
			case "failed":
				fmt.Fprintln(stdio.Stderr, "FAILED")
				testSpan.SetStatus(codes.Error, test.Message)
			case "skipped":
				testSpan.SetAttributes(attribute.Bool(telemetry.CanceledAttr, true))
				fmt.Fprintln(stdio.Stdout, "SKIPPED")
				testSpan.SetStatus(codes.Error, test.Message)
			}

			if test.Error != nil {
				fmt.Fprintln(stdio.Stderr, "ERROR:", test.Error.Error())
			}

			testSpan.End()
		}

		var errs error
		if suite.Totals.Error > 0 {
			errs = errors.Join(errs, fmt.Errorf("%d errors", suite.Totals.Error))
		}
		if suite.Totals.Failed > 0 {
			errs = errors.Join(errs, fmt.Errorf("%d failed", suite.Totals.Failed))
		}
		if errs != nil {
			suiteSpan.SetStatus(codes.Error, errs.Error())
		}

		suiteSpan.End()
	}

	time.Sleep(time.Second)

	return nil
}

// ExampleReports runs the Java tests in the example directory and returns the JUnit XML reports.
func (m *Junit) ExampleReports(
	// +defaultPath=./example/
	dir *dagger.Directory,
) *dagger.Directory {
	return dag.Container().
		From("maven:3.9-eclipse-temurin-11").
		WithMountedDirectory("/app", dir).
		WithWorkdir("/app").
		WithExec([]string{"mvn", "clean", "test"}, dagger.ContainerWithExecOpts{Expect: dagger.ReturnTypeAny}).
		Directory("/app/target/surefire-reports")
}
