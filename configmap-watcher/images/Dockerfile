FROM ${gcr_endpoint}/distroless/static

WORKDIR /

COPY dist/configmap-watcher_linux_amd64/configmap-watcher .

ENTRYPOINT ["/configmap-watcher"]
