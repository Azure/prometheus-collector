# prom-to-arm
A tool to convert [Prometheus rule group YAML](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/#configuring-rules) files to
Azure prometheus roule group [ARM template](https://learn.microsoft.com/en-us/azure/azure-resource-manager/templates/overview).

## install
Till the module is pushed to npm, install it directly from this repo:
```bash
npm i -g https://gitpkg.now.sh/Azure/prometheus-collector/tools/prom-to-arm
```

## usage
```bash
prom-to-arm -h
```
```
Usage: prom-to-arm [options] <input>

Convert Prometheus rules Yaml file to ARM template

Arguments:
  input                           Input Prometheus rule groups Yaml file path.

Options:
  -m, --mac-resource-id <string>  MAC resource id's that this rule group is scoped to.
  -c, --cluster-name <string>     The cluster name of the rule group evaluation.
  -a, --action-group-id <string>  The resource id of the action group to use for alerting rules.
  -o, --output <string>           Output path. If not set, output would be printed to std out.
  -s, --skip-validation           Skip validation.
  -h, --help                      display help for command
```
