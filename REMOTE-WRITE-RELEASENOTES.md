# Azure Monitor managed service for Prometheus remote write

## Release 02-23-2026
* Image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-remote-write/images:prom-remotewrite-20260226.1`
* Change log -
  * golang upgrade 1.25.3 -> 1.25.7
  * fix: use safe DefaultAzureCredential with RequireAzureTokenCredentials option

## Release 10-30-2025
* Image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-remote-write/images:prom-remotewrite-20251030.1`
* Change log -
  * golang upgrade 1.24.6 -> 1.25.3 (multiple CVE's fixed as part of this upgrade)
* Fixed CVEs:
    - [CVE-2025-47912](https://avd.aquasec.com/nvd/cve-2025-47912) — The `Parse` function permits values other than IPv6 addresses to be included.
    - [CVE-2025-58183](https://avd.aquasec.com/nvd/cve-2025-58183) — `tar.Reader` does not set a maximum size on the number of sparse entries.
    - [CVE-2025-58185](https://avd.aquasec.com/nvd/cve-2025-58185) — Parsing a maliciously crafted DER payload could allocate large amounts of memory.
    - [CVE-2025-58186](https://avd.aquasec.com/nvd/cve-2025-58186) — Despite HTTP headers having a default limit of 1MB, the number of headers is not constrained.
    - [CVE-2025-58187](https://avd.aquasec.com/nvd/cve-2025-58187) — Name constraint checking algorithm could incorrectly allow invalid certificates.
    - [CVE-2025-58188](https://avd.aquasec.com/nvd/cve-2025-58188) — Validating certificate chains with DSA public keys could cause excessive resource consumption.
    - [CVE-2025-58189](https://avd.aquasec.com/nvd/cve-2025-58189) — `Conn.Handshake` errors during ALPN negotiation may expose sensitive information.

## Release 08-14-2025
* Image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-remote-write/images:prom-remotewrite-20250814.1`
* Change log -
  * Updated base image `cbl-mariner/distroless/minimal` to latest security-patched version
  * Fixed Microsoft Container Registry (MCR) repo and source name in build/release pipelines
  * Various pipeline and build configuration fixes (ACR repo targeting, branch name inclusion, service tree path adjustments)
  * Upgraded golang from 1.23.x to 1.24.6 for CVE resolution
* CVE fixes
  - CVE-2025-47907

## Release 03-26-2025
* Image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-remote-write/images:prom-remotewrite-20250326.1`
* Change log -
* CVE fixes
   - CVE-2025-22870
   - CVE-2025-30204

## Release 02-14-2025
* Image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-remote-write/images:prom-remotewrite-20250214.1`
* Change log -
* CVE fixes
   - CVE-2024-45339
   - CVE-2019-11254
* golang upgrade - 1.225 -> 1.23.6

## Release 01-06-2025
* Image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-remote-write/images:prom-remotewrite-20250106.1`
* Change log -
* CVE fixes

## Release 06-17-2024
* Image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-remote-write/images:prom-remotewrite-20240617.1`
* Change log -
* CVE fixes
* golang update from 1.21.9 to 1.22.4
