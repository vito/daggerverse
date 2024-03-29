package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

type Viztest struct{}

// LogThroughput logs the current time in a tight loop.
func (m *Viztest) Spam() *Container {
	for {
		fmt.Println(time.Now())
	}
}

// Encapsulate calls a failing function, but ultimately succeeds.
func (m *Viztest) Encapsulate(ctx context.Context) error {
	_ = m.Fail(ctx, "1")
	return nil
}

func (*Viztest) LogStdout() {
	fmt.Println("Hello, world!")
}

func (*Viztest) LogStderr() {
	fmt.Fprintln(os.Stderr, "Hello, world!")
}

// Fail fails after waiting for a certain amount of time.
func (*Viztest) Fail(ctx context.Context,
	// +optional
	// +default="10"
	after string) error {
	_, err := dag.Container().
		From("alpine").
		WithEnvVariable("NOW", time.Now().String()).
		WithExec([]string{"sleep", after}).
		WithExec([]string{"false"}).
		Sync(ctx)
	return err
}
