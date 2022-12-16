# Multiarch Development

## Building Multiarch Images
Docker multiarch images are built using Docker BuildKit. We can create a builder instance:

```
docker buildx create --use
```

and then when building, use docker buildx with the same parameters as a normal docker build, plus the --platform parameter to specify which platforms the image should support:

```
docker buildx build --platform=linux/amd64,linux/arm64 .
```

## Building Binaries
To build the extra fluent-bit, otelcollector, and promconfigalidator binaries for both AMD64 and ARM64, we can use Docker multi-stage builds. BuildKit has a set of automatically defined variables that specify the build and target environments: https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope

BuildKit can use an ARM64 emulator on an AMD64 build platform so that some steps can be re-used between the ARM64 and AMD64 manifests and two different build environments do not need to be used.

The ARG instruction can take in the TARGETOS and TARGETARCH variables. Here is where the build will split off and use the emulator if the TARGETARCH is ARM64. In order to build, the package `gcc-aarch64-linux-gnu` needs to be downloaded.

All three binaries will be built in parallel. Go module packages only need to be downloaded once instead of for each architecture since the TARGETOS and TARGETARCH arguments are only used right before running `go build`. Only the go environment variables need to be changed so that go knows which architecture should be built.


## Building the Non-Distroless Image
The actual image build is the same, except for now the fluent-bit, otelcollector, and promconfigvalidator binaries will be copied over from the earlier build stages. All static files are copied over into the container. Then the ARG instruction for TARGETOS and TARGETARCH is included so that all package install commands will either be for AMD64 for ARM64. tdnf will automatically recognize the target platform and install the correct package arch from the Mariner repo. The TARGETARCH is also provided as an argument for `setup.sh`. This is only necessary if we are installing custom packages outside of the Mariner repo and need to differentiate between which to download. For example:

```
if [ "${ARCH}" != "amd64" ]; then
  wget https://github.com/microsoft/Docker-Provider/releases/download/mdsd-mac-official-06-13/azure-mdsd_1.19.3-build.master.428_aarch64.rpm
  sudo tdnf install -y azure-mdsd_1.19.3-build.master.428_aarch64.rpm
else
  wget https://github.com/microsoft/Docker-Provider/releases/download/mdsd-mac-official-06-13/azure-mdsd_1.19.3-build.master.428_x86_64.rpm
  sudo tdnf install -y azure-mdsd_1.19.3-build.master.428_x86_64.rpm
fi
```

## Building the Distroless Image
Since building the distroless image is just copying over static files from the non-distroless image, there are no changes for this part of the Dockerfile. Similarly, no changes are necessary in `main.sh`; all commands to run the packages will be the same.