package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type Viztest struct {
	Num int
}

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

func (*Viztest) PrimaryLines(n int) string {
	buf := new(strings.Builder)
	for i := 1; i <= n; i++ {
		fmt.Fprintln(buf, "This is line", i, "of", n)
	}
	return buf.String()
}

func (*Viztest) ManyLines(n int) {
	for i := 1; i <= n; i++ {
		fmt.Println("This is line", i, "of", n)
	}
}

func (vt *Viztest) ManySpans(
	ctx context.Context,
	n int,
	// +default=0
	delayMs int,
) error {
	eg := new(errgroup.Group)
	for i := 1; i <= n; i++ {
		i := i
		eg.Go(func() error {
			time.Sleep(time.Duration(i*delayMs) * time.Millisecond)
			subCtx, span := Tracer().Start(ctx, fmt.Sprintf("span %d", i))
			defer span.End()
			_, err := vt.Echo(subCtx, fmt.Sprintf("This is span %d of %d at %s", i, n, time.Now()))
			return err
		})
	}
	return eg.Wait()
}

func (*Viztest) StreamingLogs(
	ctx context.Context,
	// +optional
	// +default=5
	batchSize int,
	// +optional
	// +default=500
	delayMs int,
) {
	ticker := time.NewTicker(time.Duration(delayMs) * time.Millisecond)
	lineNo := 1
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for i := 0; i < batchSize; i++ {
				fmt.Printf("%d: %d\n", lineNo, time.Now().UnixNano())
				lineNo += 1
			}
		}
	}
}

func (*Viztest) Echo(ctx context.Context, message string) (string, error) {
	return dag.Container().
		From("alpine").
		WithExec([]string{"echo", message}).
		Stdout(ctx)
}

// Accounting returns a container that sleeps for 1 second and then sleeps for
// 2 seconds.
//
// It can be used to test UI cues for tracking down the place where a slow
// operation is configured, which is more interesting than the place where it
// is un-lazied when you're trying to figure out where to optimize.
func (*Viztest) Accounting(ctx context.Context) *Container {
	return dag.Container().
		From("alpine").
		WithEnvVariable("NOW", time.Now().String()).
		WithExec([]string{"sleep", "1"}).
		WithExec([]string{"sleep", "2"})
}

func (v Viztest) Add(
	// +optional
	// +default=1
	diff int,
) *Viztest {
	v.Num++
	return &v
}

func (v Viztest) CountFiles(ctx context.Context, dir *Directory) (*Viztest, error) {
	ents, err := dir.Entries(ctx)
	if err != nil {
		return nil, err
	}
	v.Num += len(ents)
	return &v, nil
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
