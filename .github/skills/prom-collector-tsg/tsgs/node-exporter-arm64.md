# TSG: Node Exporter Missing Labels on ARM64

- ARM64 nodes expose fewer `/proc/cpuinfo` fields than x86_64
- `node_exporter` labels like CPU model/family may be absent — this is by design
- Update dashboards/alerts to not assume architecture-specific labels
- Consider metric relabeling to add stable labels (e.g. `node_architecture`)
