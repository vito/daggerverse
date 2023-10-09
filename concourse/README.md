Assumptions:
- Docker is running locally
- OS is macOS or Linux

> **Note**
> Tested on macOS `12.7` running Docker Engine `24.0.6`

Step-by-step instructions:
1. In the parent direction, run:
```sh
git submodule init && git submodule update
```
2. Build a Dagger CLI & Engine:
```sh
cd dagger && ./hack/dev
```
3. Ensure that you are in this module's directory & using the correct Dagger CLI & Engine:
```sh
cd ../concourse && direnv allow
dagger version
dagger serve
```
4. Run this module:
```sh
dagger serve concourse.quickstart
```
5. Concourse Web UI is now available via <http://localhost:8080> 
