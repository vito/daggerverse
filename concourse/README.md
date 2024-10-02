## Assumptions

- Docker running locally
- Dagger CLI `0.13.3` or newer is installed

> **Note**
> Tested on Ubuntu `22.04` running Docker Engine `27.1.1` & Dagger `0.13.3`

## How to use this module?

```sh
# On the first run, expect this to take 1min or more, depending on your internet connection:
dagger call quickstart up
```

Concourse Web UI is now available via <http://localhost:8080>

![Concourse in Dagger](concourse.png)

## How to configure a Concourse pipeline?

> **Warning**
> Volumes in Concourse will not work correctly if you are running on macOS.

1. Download `fly` from <http://localhost:8080/api/v1/cli?arch=amd64&platform=darwin> (bottom right corner)
2. Add the `concourse.yml` pipeline from this directory (requires DockerHub credentials)

```sh
fly login -t dagger -c http://localhost:8080 -u dagger -p dagger

fly set-pipeline -c dagger.yml -p dagger -t dagger

fly unpause-pipeline -p dagger -t dagger
```

3. View this pipeline in the UI <http://localhost:8080/teams/main/pipelines/dagger?group=z>
    - Requires login with username `dagger` & password `dagger` <http://localhost:8080/sky/login>

This is what the end-result will look like:

![Dagger pipeline running in Concourse in Dagger](dagger-pipeline.png)
