# Testcontainers module

A module for running test suites that use Testcontainers. The goal is to not
require any code changes.

### Usage

For a single test suite:

```go
dag.Container().
    From("foo").
    With(dag.Testcontainers().Setup)
```

If you are running many suites, consider starting a long-running `dockerd`:

```go
if _, err := dag.Testcontainers().DockerService().Start(ctx); err != nil {
    return err
}

// continue as normal; Setup will just bind the same service

dag.Container().
    From("foo").
    With(dag.Testcontainers().Setup).
```

This will prevent wasting time restarting `dockerd` if there are long pauses
between suite runs (e.g. due to CI load). Don't worry about stopping it; it'll
be cleaned up when the function exits.

### Demos

```sh
# run all the testcontainers-go examples:
dagger -m github.com/vito/daggerverse/test call testcontainers
```
