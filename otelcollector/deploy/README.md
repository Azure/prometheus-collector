# Deployment Instructions

#### Step 0 : Pre-requisites
  You have a kubernetes cluster, which you want to monitor with this tool. You also will need [kubectl client tool](https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/) and [helm client tool](https://helm.sh/docs/intro/install/) to continue this deployment.

#### Step 1 : Create MDM Metric Account(s) & Pfx certificate for each account
  You can configure prometheus-collector to ingest different metrics into different MDM account(s). You will need to create atleast one MDM account (to use as default metric account) and have the name of that default MDM account. You also will need pfx certificate for each of the MDM accounts to which you will be configuring prometheus-collector to ingest metrics. See [configuration.md](../configuration.md) for more information about how to configure a metric account per scrape job (to ingest metrics from that scrape job to a specified metric account. If No metric account is specified as part of prometheus configuration for any scrape job, metrics produced by that scrape job will be ingested into the default metrics account specified in the configmap [step 6.1 below])

#### Step 2 : Upload Pfx certificate(s) to Azure KeyVault
  Azure KeyVault is the only supported way for this prometheus-collector to read authentication certificates for ingesting into metric store account(s). Create an Azure KeyVault (if there is not one already that you can use to store certificate(s) ). Import certificate(s) (pfx is required) per metric account into the KeyVault (ensure private key is exportable for the pfx certificate when importing into KeyVault), and update the secretProviderClass.yaml with the below (and save the secretProviderClass.yaml file)
     - KeyVaultName
     - KeyVault TenantId
     - Certificate Name (for each of the account's Pfx certificate that you uploaded to KeyVault in this step)

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
helm install csi csi-secrets-store-provider-azure/csi-secrets-store-provider-azure --namespace kube-system 
```

#### Step 5 : Apply the secretProviderClass.yaml that you updated step-2
```shell kubectl apply -f secretProviderClass.yaml ```

#### Step 6 : Update configmap to provide default MDM Account name and enable/disable default scrape targets
  Provide the default MDM account name in the config map (prometheus-collector-configmap.yaml), optionally enable/disable default scrape targets for your cluster(kubelet, coredns, etc.) using the configmap settings, and apply the configmap to your kubernetes cluster (see below steps)
     - 6.1) Ensure the line below in the configmap has your MDM account name (which will be used as the default MDM account to send metrics to)
          ``` 
            account_name = "mymetricaccountname"
          ```
     - 6.2) Specify if you'd like default kubelet or coredns scrape configs added to the prometheus yaml for you. Set to false, if you don't want these targets scraped or if you already include them in your prometheus yaml. Job names 'kubernetes-nodes' and 'kube-dns' are reserved if these are enabled.
```yaml
            default-scrape-settings: |-
              [default_scrape_settings]
                kubelet_enabled = true
                coredns_enabled = true
```
     - 6.3) Apply the configmap to the cluster
```shell
            kubectl apply -f prometheus-collector-configmap.yaml
```

#### Step 7 : Provide Prometheus scrape config
Provide the prometheus scrape config as needed in the prometheus configmap. See [configuration.md](../configuration.md) for more tips on the prometheus config. There are two ways of doing so:
**Use the provided configmap (prometheus-config.yaml) as starting point, and make changes as needed to the prometheus-config.yaml configmap and apply:**
```shell
        kubectl apply -f prometheus-collector-configmap.yaml
```

By default and for testing purposes, the provided configmap has scrape config to scrape our reference service (weather service), which is located in the [app](../app/prometheus-reference-app.yaml) folder. If you'd like to use the default scrape config, you need to deploy the weather service app by running the following command while in the [app](../app/prometheus-reference-app.yaml) folder:
```shell
    kubectl apply -f prometheus-reference-app.yaml
```
    
**If you have your own prometheus yaml scrape configuration and want to use that without having to paste into the configmap, rename your config file   to ```prometheus-config``` and run:**
```shell
       kubectl create configmap prometheus-config --from-file=prometheus-config -n kube-system
```

**Tip** We will validate provided prometheus configuration using [promtool](https://github.com/prometheus/prometheus/tree/main/cmd/promtool), an official commandline prometheus tool, with the command:
```shell
    promtool check config <config name>
```
    You can also download to this tool and run this command for your prometheus config before adding to the configmap, to save some time.

#### Step 8 :  Deploy the Prometheus-collector agent
Provide your cluster name as value for ```CLUSTER``` environment variable in deployment file (prometheus-collector.yaml). This will be added as a label ```cluster``` to every metric collected from this cluster. Now you are ready to deploy the prometheus collector (prometheus-collector.yaml) by running the below kubectl commsnd. [Prometheus-collector will run in kube-system namespace as a singleton replica]
```shell kubectl apply -f prometheus-collector.yaml ```
