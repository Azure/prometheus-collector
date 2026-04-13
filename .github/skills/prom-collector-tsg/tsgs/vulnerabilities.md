# TSG: Vulnerabilities / CVEs

1. Run trivy scan via GitHub action: https://github.com/Azure/prometheus-collector/actions/workflows/scan.yml
2. If CVEs are in base image → create release with new image build (Mariner base auto-upgrades)
3. If CVEs are in packages → check version against Mariner CVE database at aka.ms/astrolabe
4. If we have same or higher version → false positive
