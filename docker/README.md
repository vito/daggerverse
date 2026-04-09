# Docker module

A module for all things Docker.

* [x] Docker in Docker
* [x] Docker Compose

## Docker in Docker (DinD)

### Quick Start

```go
// Use default configuration (Docker 28, no cache)
service := dag.Docker().Daemon().Service()
```

### Configuration Options

**Specify Docker version:**
```go
dag.Docker().Daemon().WithVersion("29").Service()
```

**Custom cache volume:**
```go
dag.Docker().Daemon().
    WithCache(dag.CacheVolume("my-docker-cache")).
    Service()
```

**Custom storage driver:**
```go
dag.Docker().Daemon().
    WithStorageDriver("vfs").  // Options: vfs, overlay2, native
    Service()
```

### Docker 29 Compatibility

This module **defaults to Docker 28** for stability. Docker 29 introduced breaking
changes with the containerd snapshotter that cause issues in nested DinD scenarios.

**Version-specific behavior:**

**Docker 28 and earlier:**
- No automatic cache volume (uses overlay2 directly on container filesystem)
- No special flags needed
- **Fast and works out of the box**

**Docker 29 and later:**
- Automatically creates cache volume at `/var/lib/docker` (prevents nested overlay issues)
- Adds `--feature containerd-snapshotter=false` flag
- Falls back to `vfs` storage driver if needed
- **Slower but guaranteed to work in nested DinD scenarios**

Users can always override with `WithCache()` or `WithStorageDriver()` for custom configurations.

See [docker/cli#6646](https://github.com/docker/cli/issues/6646) for details on Docker 29 issues.

## Docker Compose

### Demos

```sh
# via CLI:
dagger -m github.com/vito/daggerverse/docker \
    up --native \
    compose \
        --dir https://github.com/vito/dagger-compose \
        --files wordpress.yml \
    all

# or:
dagger -m github.com/vito/daggerverse/test up wordpress --native
```
