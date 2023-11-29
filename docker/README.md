# Docker module

A module for all things Docker.

* [x] Docker in Docker
* [x] Docker Compose

### Demos

```sh
dagger up compose --dir https://github.com/vito/daggerverse/test --file wordpress.yml all --native

# or:
dagger -m ../test up wordpress --native
```
