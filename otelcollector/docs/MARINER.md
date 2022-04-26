# Mariner Development
## Links
* Official eng.hub docs: [aka.ms/mariner](aka.ms/mariner)
* View current [container images](https://eng.ms/docs/products/mariner-linux/gettingstarted/containers/marinercontainerimage)
* View available [packages](https://eng.ms/docs/products/mariner-linux/gettingstarted/packages/packagesx) and how to request additional ones

## Mariner vs. Ubuntu
Mariner is an RPM-based distro whereas Ubuntu is Debian-based. Mariner uses `tdnf` as it's package manager whereas Ubuntu uses `apt`. The largest differnce between the two then is replacing all the `apt` commands with `tdnf`. [This table](https://eng.ms/docs/products/mariner-linux/gettingstarted/ubuntu/atlas#command-replacement-reference-table) provides all the equivalent commands between the two. Not all commands are available with `tdnf`, which is a trimmed-down C based version of the `dnf` package manager. `dnf` can be installed by `tdnf` with `tdnf install -y dnf`.

More info about `apt` vs `tdnf` can be found [here](https://eng.ms/docs/products/mariner-linux/onboarding/packaging/packagemanagement).

## Adding an RPM Repository
New repositories are added with a `*.repo` file stored in the `/etc/yum.repos.d/` directory.

Mariner by default has certain repo files in the container already. These include the repos for the mariner base and mariner extended packages.

MetricsExtension is included in the mariner-official-extra repository and MDSD is in the azurecore repository. Both have corresponding repo files at [here](/otelcollector/build/linux/).

## Distroless Containers

Distroless containers exclude packages provided by the distro like a shell and package manager. They provide a slimmed down image with a smaller attack surface. We'll still need a shell for our main.sh script and any debugging done by exec-ing into the container. But a distroless base image is still useful to not have the package managers and any unecessary packages just needed for building. 

### Docker Multi-Stage Builds
Since the distroless container does not have a package manager, we will still need a way to install all the necessary packages. The standard way to do this is through [multi-stage builds](https://docs.docker.com/develop/develop-images/multistage-build/). The first build uses the regular base container and sets up everything as normal up until adding the entry command as running the `main.sh` script.

The second build uses the distroless image as the base image. Only the files we need at runtime are copied over from the first build. An example of this is [here](https://medium.com/@alexanto222/hardening-of-docker-images-distroless-images-d6d87b591a59).

Note that this means that any new files added while developing will need to be copied over to the Distroless build stage. Everything under `/opt` is currently copied over. See below for the steps to take when adding a new package.


### Using the Shell in the Distroless Container
`bash` is still copied over into our container to be able to run our `main.sh` bash script. However, there are some interface issues with `bash` and `vim` on the distroless container. `busybox` and it's shell are more common on smaller container images like `apline` and Mariner has support for a debug container that includes `busybox`. This however changes some of the commands when exec-ing into the container. Feel free to add more here if you find them.

  | Old Command | New Command |
  | --- | --- |
  | `kubectl exec -it <pod name> -- bash` | `kubectl exec -it <pod name> -- sh` |
  | `vim` | `vi` |
  | `ps -aux` | `ps` |


## Adding a New Package
### Base Image
You can start by including `tdnf install -y <package name>` in the `builder` Docker stage and trying to build to see if the package is available. `tdnf` will search all repos for the package and print out an error if it cannot be found.

Some naming conventions or package names are different for RPM packages. For example, `*-dev` and `*-debug` for Ubuntu packages vs. `*-devel` and `*-debugsymbol` for RPM pacakges. Similarly, `cron` has an equivalent RPM package called `cronie` and the name of the executable is `crond`. The package may be provided by Mariner but under a different name. `libre2` is just `re2` in the Mariner repository.

`dnf` has a command called `whatprovides` to help with this as explained [here](https://eng.ms/docs/products/mariner-linux/onboarding/packaging/packagemanagement#finding-the-right-package), but usually a google search will also work.

### Distroless Image
In the base image, run:
  * `whereis <executable>`
  * `ldd <executable path>`
  * This will print out something similar to:

    ```
    linux-vdso.so.1 (0x00007ffc2eb6e000)
    libselinux.so.1 => /lib/libselinux.so.1 (0x00007f7677a6a000)
    libpam.so.0 => /lib/libpam.so.0 (0x00007f7677a58000)
    libc.so.6 => /lib/libc.so.6 (0x00007f7677856000)
    libpcre.so.1 => /lib/libpcre.so.1 (0x00007f76777df000)
    /lib64/ld-linux-x86-64.so.2 (0x00007f7677b30000)
    libaudit.so.1 => /lib/libaudit.so.1 (0x00007f76777ae000)
    ```

  * Packages without `=>` do not need to be copied as these will already be present in the distroless container.
  * Note: for `/lib/libc.so.6`, I was having issues copying this over for Mariner v2. This package was already there in the distroless container and there were some conflicting issues when copying it over again. This was the only `.so` file I was seeing issues with.

In the Dockerfile in the distroless stage:
  * Copy over the executable file path
  * Copy over the full path of all `.so` files that have `=>` except for `/lib/libc.so.6`
  * If not all necssary `.so` files are available, there will be an error saying that file is missing during runtime.

## Our Current Package Dependencies
* `telegraf` and `fluent-bit` are both built and published by the Mariner team in the [Mariner base repository](https://packages.microsoft.com/cbl-mariner/2.0/preview/base/x86_64/). [ARM64 versions](https://packages.microsoft.com/cbl-mariner/2.0/preview/base/aarch64/) are available also.
* `MetricsExtension` is published by their team to the Mariner extras repository. [Mariner 1.0](https://packages.microsoft.com/cbl-mariner/1.0/prod/extras/x86_64/rpms/) is available. Mariner 2.0 is in progress.
* `MDSD` is published by their team to the Azure Core repository. [Release notes](https://eng.ms/docs/products/geneva/collect/instrument/linux/releasenotes) and [instructions](https://eng.ms/docs/products/geneva/getting_started/environments/linuxvm) for installing the RPM package.