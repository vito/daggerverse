Assumptions:
- Docker is running locally
- OS is Linux (iptables issues on macOS)

> **Note**
> Tested on Debian `v12.1` running Docker Engine `24.0.6`

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

💥 When running on macOS 12.7
```
...
containerd-garden-backend exited with error:
    setup host network failed:
        create chain or flush if exists failed:
            running [/usr/sbin/iptables -t filter -N CONCOURSE-OPERATOR --wait]: exit status 3:
                iptables v1.8.7 (legacy): can't initialize iptables table `filter':
                    iptables who? (do you need to insmod?)

Perhaps iptables or your kernel needs to be upgraded.
```
