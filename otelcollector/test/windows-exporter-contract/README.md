# Windows exporter compatibility validation

Run the regression checks with Python 3 and `promtool`:

```sh
python3 otelcollector/test/windows-exporter-contract/test_contract.py --promtool /path/to/promtool
```

The build pipeline runs these checks using its pinned Prometheus version. They evaluate the actual shipped recording-rule expressions against uptime-only legacy exporters, preview boot-time names, native Windows 2025 names, and overlapping aliases. They also exercise memory-usage dashboard expressions, check that disk-byte metrics survive minimal ingestion, and remove each of the 25 native metric families and six container recording outputs from one of two nodes to verify the CI alert detects the loss. Health fixtures cover a healthy Windows 2022 node alongside a down or undiscovered Windows 2025 exporter, comparing both the Ginkgo and alert predicates against the expected Windows-node inventory.

Use `--templates FILE...` for downstream JSON or TypeScript recording-rule definitions and `--dashboards FILE...` for managed Grafana JSON dashboards. No cluster credentials are required for these local checks.

## Live validation: 2026-09-04

Validated Kubernetes 1.36.3 with Windows Server 2025 Datacenter, FIPS enabled, `Standard_D2s_v3`, node image `AKSWindows-2025-gen2-26100.33296.260812`, containerd `2.0.4+azure`, AKS Windows extension 2.149 and exporter v0.31.2.

- Port 19182 served metrics; no exporter was installed on 9182. This run did not compare two running exporters. The 9182 configuration escape hatch is covered by unit tests.
- 19 of the original 24 keeplist names were present. The boot-time and physical-memory aliases were present; both container-network families were absent.
- A running Windows 2025 workload had an HNS endpoint and matching container IDs in exporter and kube-state-metrics data. The container collector reported success on repeated scrapes, including five consecutive checks after startup.
- Managed Prometheus ingested the surviving metrics, and CPU/memory container joins and recording outputs returned data. Both container-network recording outputs remained absent.
- The current CLI-created node-count and total-memory rules returned no data. A temporary managed rule group using the boot/memory fallbacks produced valid node-count, memory-total, memory-utilization and UX-memory results.
- All 65 queries from the five managed Windows dashboards executed against the workspace: 57 returned data and eight were empty because of the memory-name or container-network gaps. This validates PromQL results, not rendered Grafana UI behavior.
- Disk read/write byte families were present and powered the throughput panels when retained by the scrape keeplist. They are now included in the minimal profile and native contract.

The released AMA image was used with a custom 19182 scrape job carrying the proposed keeplist. This was not an end-to-end run of the PR image or its Ginkgo suite.

## Remaining native exporter dependency

`windows_container_network_receive_bytes_total` and `windows_container_network_transmit_bytes_total` remain required. Do not remove them from the tests or keeplist to make Windows 2025 pass: their absence breaks existing pod-network panels and recording outputs. The native exporter team must restore the contract or agree a coordinated compatibility change before default-on/GA approval. The Windows 2025 Ginkgo contract will fail while this gap remains.

Native extension deployment normally takes 30–60 minutes. Wait for that window and verify `AKSWindowsExtension` on the VMSS instance before diagnosing a missing listener. In this run the extension was installed manually before the window elapsed; later platform reconciliation and successful enable operations were observed. Automatic installation on an untouched node was therefore not proved, and the initial delay is not a rollout blocker.

## CI cluster migration

The CI template replaces the Windows 2019 pool with Windows 2025/FIPS on a Gen2-capable VM size and a managed OS disk. Windows 2022 coverage remains. Deploy on an AKS version supporting Windows 2025 (1.32 or later); the region's default version is used unless the cluster already exists. For existing CI clusters, coordinate retirement of the old pool with the cluster owner: an incremental ARM deployment does not delete an omitted Windows 2019 pool. Windows 2019 container-image builds remain unchanged because their consumers extend beyond this AKS test cluster.

The per-node metric-contract alert uses explicit metric selectors. A metric-name regex selector returned HTTP 501 from the validation workspace, while the replacement query succeeded and identified the Windows 2025 node with the missing metrics. Its 30-minute pending period avoids alerting on brief scrape/recording-rule warmup.

The separate target-health alert compares Windows nodes in `kube_node_info` with healthy exporter instances, so a healthy node cannot hide a down or undiscovered exporter on another node. Its one-hour pending period accommodates native extension deployment. Ginkgo uses the same inventory check and requires a nonempty Windows-node inventory. Run the full Windows suite after provisioning and the native exporter rollout window; its metric assertions are not a provisioning-readiness wait.
