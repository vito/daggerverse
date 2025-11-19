# Docker module

A module for all things Docker.

* [x] Docker in Docker
* [x] Docker Compose

## Docker in Docker (DinD)

### Quick Start

```go
// Use default configuration (Docker 28, auto cache)
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

**The module automatically:**
- Creates a cache volume at `/var/lib/docker` (prevents nested overlay issues)
- Disables `containerd-snapshotter` for Docker 29+ compatibility
- Allows storage driver override for advanced use cases

See [docker/cli#6646](https://github.com/docker/cli/issues/6646) for details.

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
