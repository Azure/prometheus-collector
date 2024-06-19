FROM mcr.microsoft.com/oss/go/microsoft/golang:1.21

RUN go install -v github.com/onsi/ginkgo/v2/ginkgo@latest

RUN curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash \
    && helm version

RUN apt-get update && apt-get -y upgrade && \
    apt-get -f -y install curl apt-transport-https lsb-release gnupg python3-pip && \
    curl -sL https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > /etc/apt/trusted.gpg.d/microsoft.asc.gpg && \
    CLI_REPO=$(lsb_release -cs) && \
    echo "deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ ${CLI_REPO} main" \
    > /etc/apt/sources.list.d/azure-cli.list && \
    apt-get update && \
    apt-get install -y azure-cli && \
    rm -rf /var/lib/apt/lists/*

RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
RUN install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

COPY arc-conformance/e2e_tests.sh /
COPY ginkgo-e2e ginkgo-e2e/
RUN ginkgo build -r ./ginkgo-e2e
RUN mkdir ginkgo-test-binaries && mv ginkgo-e2e/containerstatus/containerstatus.test ginkgo-e2e/livenessprobe/livenessprobe.test ginkgo-e2e/operator/operator.test ginkgo-e2e/prometheusui/prometheusui.test ginkgo-e2e/querymetrics/querymetrics.test ginkgo-test-binaries/

RUN ["chmod", "+x", "/e2e_tests.sh"]
ENTRYPOINT [ "/bin/bash" ]
CMD [ "/e2e_tests.sh" ]