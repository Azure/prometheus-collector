# ConfigMap Watcher

Sidecar to sync ConfigMap changes to a file.

## Build & Push image

* Set image name (i.e export `IMG_NAME=mycontainerregistry.azurecr.io/configmap-watcher:latest`):
    ```shell
    export IMG_NAME=<image_name>
    ```
* Build and push image:
    ```shell
    make build package docker-build docker-push 
    ```