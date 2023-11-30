# Docker module

A module for all things Docker.

* [x] Docker in Docker
* [x] Docker Compose

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
