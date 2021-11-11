- **Main branch builds:** ![Builds on main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event=push)

- **PR builds:** ![PRs to main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event!=push)


# Deployment Instructions

#### Step 0 : Pre-requisites
  You have a kubernetes cluster, which you want to monitor with this tool. You also will need [kubectl client tool](https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/) and [helm client tool(v3.7.0 or later - see below note)](https://helm.sh/docs/intro/install/) to continue this deployment. 
  
  ```Note: Our charts will not work on HELM clients < 3.7.0```

  ```Note: Its recommended to use linux/WSL in windows to deploy with the below steps. Though windows command shell/powershell should work, we haven't fully tested with them. If you find any bug, please let us know (askcoin@microsoft.com)```

#### Step 1 : Create MDM Metric Account(s) & Pfx certificate for each account
  You can configure prometheus-collector to ingest different metrics into different MDM account(s). You will need to create atleast one MDM account (to use as default metric account) and have the name of that default MDM account. Logs account is not needed for collecting Prometheus metrics. You also will need pfx certificate for each of the MDM accounts to which you will be configuring prometheus-collector to ingest metrics. See [configuration.md](../configuration.md) for more information about how to configure a metric account per scrape job (to ingest metrics from that scrape job to a specified metric account. If no metric account is specified as part of prometheus configuration for any scrape job, metrics produced by that scrape job will be ingested into the default metrics account specified in the configmap [step 6.1 below])

#### Step 2 : Upload/provision certificate(s) for your metric store account(s) in Azure KeyVault
  Azure KeyVault is the only supported way for this prometheus-collector to read authentication certificates for ingesting into metric store account(s). Create an Azure KeyVault (if there is not one already that you can use to store certificate(s) ). Import/create certificate(s) (private key should be exportable) per metric account into the KeyVault (ensure private key is exportable for the certificate), and update the secretProviderClass.yaml with the below (and save the secretProviderClass.yaml file)
     - KeyVaultName
     - KeyVault TenantId
     - Certificate Name (for each of the account's certificate (thats exportable with private key) that you uploaded to KeyVault in this step)

#### Step 3 : Provide access to KeyVault using service principal
  Prometheus-collector will need a service principal and secret to access key vault and pull the certificate(s) to use for ingesting metrics into MDM account(s). For this purpose, you will need to create/use a service principal and do the following -
     - 3.1) Create a new service principal & secret (or) use an existing service principal with its secret
     - 3.2) For the KeyVault resource, grant 'Key Vault Secrets User' built-in role for your service principal (from step 3.1)
     - 3.3) Copy the service principal app/clientid & its secret
     - 3.4) Create a kubernetes secret in your cluster for the above service principal and its secret (from step 3.3 above)
        ```
        kubectl create secret generic akv-creds --from-literal clientid="<service_principal_client_id>" --from-literal clientsecret="<service_principal_client_secret>" -n=kube-system 
        ```

#### Step 4 : Install csi driver & secrets store provider for azure KeyVault in your cluster
```shell 
helm repo add csi-secrets-store-provider-azure https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/charts 
```
```shell 
helm upgrade --install csi csi-secrets-store-provider-azure/csi-secrets-store-provider-azure --set secrets-store-csi-driver.enableSecretRotation=true --namespace kube-system 
```

#### Step 5 : Apply the secretProviderClass.yaml that you updated step-2
```shell kubectl apply -f secretProviderClass.yaml ```

#### Step 6 : Update configmap to provide default MDM Account name, enable/disable default scrape targets or allow only the metric names matching regexes for default targets 

Provide the default MDM account name in the config map (prometheus-collector-settings-configmap.yaml), optionally enable/disable default scrape targets for your cluster(kubelet, coredns, etc.) using the configmap settings or optionally configure certain metric(s) collection from default targets using regex based filtering, and apply the configmap to your kubernetes cluster (see below steps)

- 6.1) Ensure the line below in the configmap has your MDM account name (which will be used as the default MDM account to send metrics to)

     ```yaml
    prometheus-collector-settings: |-
      default_metric_account_name = "mymetricaccountname"
     ```

- 6.2) Specify if you'd like default kubelet or coredns scrape configs added to the prometheus yaml for you. Set to false, if you don't want these targets scraped or if you already include them in your prometheus yaml. Job names `kubelet`, `cadvisor`, `kube-dns`, `kube-proxy`, `kube-apiserver`, `kube-state-metrics` and `node` are reserved if these are enabled.

    ```yaml
    default-scrape-settings-enabled: |-
      kubelet = true
      coredns= true
      cadvisor = true
      kubeproxy = true
      apiserver = true
      kubestate = true
      nodeexporter = true
    ```

- 6.3) Specify if you'd like to filter out metrics collected for the default targets using regex based filtering.

    ```yaml
    default-targets-metrics-keep-list: |-
      kubelet = "<regex>"
      coredns= "<regex>"
      cadvisor = "<regex>"
      kubeproxy = "<regex>"
      apiserver = "<regex>"
      kubestate = "<regex>"
      nodeexporter = "<regex>"
      windowsexporter = "<regex>"
      windowskubeproxy = "<regex>"
    ```
  Note that if you are using  
      1. quotes in the regex you will need to escape them using a backslash. Example - keepListRegexes.kubelet = `"test\'smetric\"s\""`  
      2. backslash in the regex, you will need to replace a single instance of `\` with `\\\\\\\\\`. This is because of the multiple environment variable substitutions that happen before this goes into effect in the configuration.
  
- 6.3) Apply the configmap to the cluster
    ```shell
    kubectl apply -f prometheus-collector-settings-configmap.yaml
    ```

#### Step 7 : Provide Prometheus scrape config
Provide more prometheus scrape config as needed as a configmap in addition to default scrape config. See [sample-scrape-configs](./sample-scrape-configs/README.md) for more tips on the prometheus config. There are two ways of doing so:
**Use the provided configmap [prometheus-config-configmap.yaml](./sample-scrape-configs/prometheus-config-configmap.yaml) as starting point, and make changes as needed to the prometheus-config-configmap.yaml configmap and apply:**
```shell
        kubectl apply -f prometheus-config-configmap.yaml
```

By default and for testing purposes, the provided configmap has scrape config to scrape our reference service (weather service), which is located in the [app](../app/prometheus-reference-app.yaml) folder. If you'd like to use the default scrape config, you need to deploy the weather service app by running the following command while in the [app](../app/prometheus-reference-app.yaml) folder:
```shell
    kubectl apply -f prometheus-reference-app.yaml
```
    
**If you have your own prometheus yaml scrape configuration and want to use that without having to paste into the configmap, rename your scrape configuration file to ```prometheus-config``` and run te below command. See the provided sample prometheus scrape config [prometheus-config](./sample-scrape-configs/prometheus-config) as an example.
```shell
       kubectl create configmap prometheus-config --from-file=prometheus-config -n kube-system
```

**Tip** We will validate provided prometheus configuration using promconfigvalidator, a commandline prometheus config validation tool, with the command:
```shell
    ./promconfigvalidator --config "config-path" --otelTemplate "collector-config-template-path"
```
    You can also copy this tool and the collector config template using kubectl cp from paths /opt/promconfigvalidator and /opt/microsoft/otelcollector/collector-config-template.yml from within the prometheus-collector container and run this command for your prometheus config before adding to the configmap, to save some time.
    This by default generates the otel collector configuration file 'merged-otel-config.yaml' if no paramater is provided using the optional --output paramater.
    This is the otel config that will be applied to the prometheus collector which includes the custom prometheus config

**Note** The job names `kubelet`, `cadvisor`, `kube-dns`, `kube-proxy`, `kube-apiserver`, `kube-state-metrics`, `node`, `prometheus_collector_health`, `windows-exporter(disabled by default)` and `kube-proxy-windows(disabled by default)` are reserved and if they were to be present in the custom configuration, the otelcollector will fail to start because of the duplicate job name. Please refrain from using these for the job names. If you were to use these, please disable the corresponding default targets as mentioned in the previous section and then you can use these names as the job names in the custom prometheus configuration.

#### Step 8 : Deploy prometheus-node-exporter and kube-state-metrics in your cluster

- See steps [here](./sample-scrape-configs/README.md) to deploy prometheus-node-exporter and kube-state-metrics
- You don't need to specify scrape configs for prometheus-node-exporter and kube-state-metrics as its included in the default scrape targets (unless you 
  turned it OFF in the config map using the settings ```scrapeTargets.kubestate``` and ```scrapeTargets.nodeexporter```)
- Make note of the release names for kube-state-metrics and prometheus-node-exporter deployments from this step, as you will need them for the next step

#### Step 9 :  Deploy the Prometheus-collector agent
Provide your cluster name as value for ```CLUSTER``` environment variable in deployment file (prometheus-collector.yaml). This will be added as a label ```cluster``` to every metric collected from this cluster. 
Provider your release names for kube-state-metrics and node-exporter deployments in the env vars '
Now you are ready to deploy the prometheus collector (prometheus-collector.yaml) by running the below kubectl commsnd. [Prometheus-collector will run in kube-system namespace as a singleton replica]
```shell kubectl apply -f prometheus-collector.yaml ```
