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

### Docker 29 Compatibility

This module defaults to **Docker 28** for stability. Docker 29 introduced breaking
changes that cause overlay mount failures in nested Docker-in-Docker scenarios
(see [docker/cli#6646](https://github.com/docker/cli/issues/6646)).

**Default behavior** (Docker 28, works out of the box):
```go
dag.Testcontainers().Setup(ctr)
```

**Opting into Docker 29:**
```go
dag.Testcontainers().
    WithDockerVersion("29").
    Setup(ctr)
```

**Custom storage driver** (if you encounter overlay issues):
```go
dag.Testcontainers().
    WithDockerStorageDriver("vfs").  // Slower but always works
    Setup(ctr)
```

**Version-specific behavior:**

**Docker 28 (default):**
- Fast - uses overlay2 directly without cache overhead
- No special configuration needed

**Docker 29+:**
- Automatically creates cache volume at `/var/lib/docker`
- Disables containerd-snapshotter with `--feature containerd-snapshotter=false`
- Slower but guaranteed to work in nested scenarios

### Demos

```sh
# run all the testcontainers-go examples:
dagger -m github.com/vito/daggerverse/test call testcontainers
```
