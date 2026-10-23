# Container Insights reference app

Go-only variant of the existing reference app for Linux/amd64 CI clusters.
Pod annotations use existing Container Insights annotated-pod discovery on port
2112. No Service, scrape-config edit, or Python sidecar is needed.
Annotated-pod scraping must already be enabled. The legacy RPC app was retired
from both Model clusters on October 20, 2026; scanner records may lag, and this
is not an S360 closure claim.

Resolve the approved published build's Go reference image to an immutable
`repository@sha256:...` before substituting `REFERENCE_APP_GOLANG_IMAGE` in
`deployment.yaml`; never use a floating `latest`. The collector build pipeline
publishes `<build-version>-ref-app-golang` and records its digest in the
`refappgolanglinux` metadata/signing payload. A versioned tag alone is not a
compliance assertion: assess the resolved artifact's scan, runtime support and
provenance before deploying it.

After deployment, use the existing Azure CLI login and isolated kubeconfig:

```sh
python3 check_ingestion.py --self-test
python3 check_ingestion.py \
  --subscription "$SUBSCRIPTION_ID" --workspace "$WORKSPACE_CUSTOMER_ID" \
  --cluster-resource-id "$AKS_RESOURCE_ID" \
  --kubeconfig "$KUBECONFIG" --context "$KUBE_CONTEXT" --since "$DEPLOYMENT_START_UTC"
```

The check waits at most five minutes for Ready pods and fresh
`myapp_temperature` **and** `myapp_measurements_total` in `InsightsMetrics` for
each exact annotated pod URL and cluster. Authentication/query errors fail
immediately. The self-test rejects missing families, old pods, wrong routes and
zero rows. This check does not query Azure Monitor workspace PromQL.

The initial approved image is the existing mirror
`referenceapps/golang@sha256:493a22d616c35aedc5ea2a90851ad83caaa03d4b46dda3cd9d8026d2fca84517`.
Its October 20, 2026 Trivy scan detected no vulnerabilities across 14 Azure Linux
3.0 and 31 Go-binary package entries, including Go 1.26.9. The generated
CycloneDX inventory is not a signed attestation or S360 clearance, and this
custom CI image is not claimed to be a supported Microsoft production release.
