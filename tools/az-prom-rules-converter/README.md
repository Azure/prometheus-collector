# az-prom-rules-converter
A tool to convert [Prometheus rules YAML file](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/#configuring-rules) files to
Azure Prometheus rule groups [ARM template](https://learn.microsoft.com/en-us/azure/azure-resource-manager/templates/overview).
Currently support [API version 2023-03-01](https://learn.microsoft.com/en-us/rest/api/monitor/prometheusrulegroups/prometheus-rule-groups).

## install 
### Prerequisite
Install nodejs LTS version:
https://nodejs.org/en/

### Install package globaly 
Install it directly from this repo.
We use [gitpkg](https://gitpkg.vercel.app/) as a workaround to install a github repository sub folder as an npm package: 
```bash
npm i -g https://gitpkg.now.sh/Azure/prometheus-collector/tools/az-prom-rules-converter?main
```

## usage
```bash
az-prom-rules-converter -h
```
```
Usage: az-prom-rules-converter [options] <input>

Convert Prometheus rules Yaml file to ARM template

Arguments:
  input                           Input Prometheus rule groups Yaml file path.

Options:
  -amw, --azure-monitor-workspace <string>  Azure monitor workspace id's that this rule group is scoped to.
  -c, --cluster-name <string>               The cluster name of the rule group evaluation.
  -a, --action-group-id <string>            The resource id of the action group to use for alerting rules.
  -o, --output <string>                     Output path. If not set, output would be printed to std out.
  -s, --skip-validation                     Skip validation.
  -l, --location <string>                   Rule group location.
  -h, --help                                display help for command
```

## usage with pipeline
```bash
Get-Content .\examples\yaml-example1.yml | node az-prom-rules-converter
```