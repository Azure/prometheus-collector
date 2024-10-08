# Node Mixin

Last synced on 07/28/2022 to commit - https://github.com/prometheus/node_exporter/commit/7519830a8a9fb7e2321655031692526498f7cb9a

_This is a work in progress. We aim for it to become a good role model for alerts
and dashboards eventually, but it is not quite there yet._

The Node Mixin is a set of configurable, reusable, and extensible alerts and
dashboards based on the metrics exported by the Node Exporter. The mixin creates
recording and alerting rules for Prometheus and suitable dashboard descriptions
for Grafana.

To use them, you need to have `jsonnet` (v0.16+) and `jb` installed. If you
have a working Go development environment, it's easiest to run the following:

```bash
go install github.com/google/go-jsonnet/cmd/jsonnet@latest
go install github.com/google/go-jsonnet/cmd/jsonnetfmt@latest
go install github.com/jsonnet-bundler/jsonnet-bundler/cmd/jb@latest
```

Next, install the dependencies by running the following command in this
directory:

```bash
jb install
```

You can then build the Prometheus rules files `node_alerts.yaml` and
`node_rules.yaml`:

```bash
make node_alerts.yaml node_rules.yaml
```

You can also build a directory `dashboard_out` with the JSON dashboard files
for Grafana:

```bash
make dashboards_out
```

Note that some of the generated dashboards require recording rules specified in
the previously generated `node_rules.yaml`.

For more advanced uses of mixins, see
<https://github.com/monitoring-mixins/docs>.
