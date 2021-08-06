> [!Note]
> Prometheus metrics in MDM is still in active development. It is only available for a very small set of customers to provide very early feedback - limited private preview. Geneva will open this up for broader preview, after we've had a chance to address feedback received in the current limited preview. If your team has not already been contacted for the limited preview, then you are not yet eligible for this preview. You can also join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly.

# Create metrics account and set up KeyVault authentication

## Create metrics account

Prometheus metrics that you want to work with, will be stored in a Geneva metrics (MDM) account. If you don't have an existing metrics account in Geneva, follow the instructions to create a [metrics account](~/getting_started/v2/createaccounts_basic.md). Logs account is not needed for Prometheus metrics collection.

If you are part of the [Limited Preview](~/metrics/Prometheus/PromMDMLimitedPreview.md) , please create a new account in the 'Prom1' shared stamp, as this has been specifically set up for Limited preview evaluations.

To create account in the 'Prom1' stamp, specify that stamp explicitly during the account creation wizard, as shown below  

![Prom1](~/metrics/images/prometheus/PromCreateAccountLimitedPreview.png)

## Set up KeyVault authentication

Prometheus metrics will be collected from your Kubernetes cluster by an agent, and stored in the Geneva Metrics (MDM) account you created. To ensure the agent can authenticate to Geneva Metrics (MDM), we will set up KeyVault authentication with an RBAC-enabled KeyVault. This will involve, creating the certificate and making it available to the agent (client side) and MDM (server side).

At this point, only KeyVault authentication is supported for Prometheus metrics ingestion.  

### Create certificate

To create our KeyVault certificate, follow the guidance outlined in [Create a new Azure KeyVault (AKV) certificate](~/collect/authentication/keyvaultcreatecert.md)

##### Save certificate details

With your certificate created, you want to save some information that we will need in subsequent steps. Specifically,  

- KeyVault Name
- KeyVault TenantId
- Certificate Name  

### Set up permissions for agent to pull certificate from KeyVault

Next, lets ensure the agent has the right authorization to pull the certificate from KeyVault. To do this we will need to use a service principal.

To do so using the Azure Portal:

* You can [create a new service principal & secret](https://docs.microsoft.com/en-us/azure/active-directory/develop/howto-create-service-principal-portal), OR use an existing service principal with its secret.  

* Once you have a service principal and secret, it needs to be tagged as a valid KeyVault user. To do this,  
    - Go to the KeyVault resource  
    - Grant your service principal the 'Key Vault Secrets User' built-in role  

Alternatively, you can use a CLI:
* Replace `<service prinipal name>` with the name you'd like for the service principal. From the values printed out, the `appId` value is the service principal client ID. The `password` is the service principal secret. Save these for the prometheus-collector deployment in the Deploy Agent step.

    ```bash
    az ad sp create-for-rbac --skip-assignment --name <service principal name>
    ```
*  Then run the following and replace the `<service principal appId>` with the value from the step above and specify the resource ID of the KeyVault for the `--scope` parameter:
    ```bash
    az role assignment create --role "Key Vault Secrets User" --assignee <service principal appId> --scope /subscriptions/<subscriptionid>/resourcegroups/<resourcegroup>/providers/Microsoft.KeyVault/vaults/<keyvaultname>
    ```

Now, save the service principal app/clientID & its secret, as we will need this in a subsequent step.  

### Install Azure KeyVault driver in your cluster

To be able to mount secrets and certificates as storage volumes, you will need to install the csi driver & secrets store provider for Azure KeyVault in your cluster.  
For this we will leverage [HELM](https://kubernetes.io/blog/2016/10/helm-charts-making-it-simple-to-package-and-deploy-apps-on-kubernetes/). The following commands can be used for this. See an example of this below.  

```shell
helm repo add csi-secrets-store-provider-azure https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/charts 
helm upgrade --install csi csi-secrets-store-provider-azure/csi-secrets-store-provider-azure --set secrets-store-csi-driver.enableSecretRotation=true --namespace <my_any_namespace> --create-namespace
```

**Example** :-

```shell
helm repo add csi-secrets-store-provider-azure https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/charts
helm upgrade --install csi csi-secrets-store-provider-azure/csi-secrets-store-provider-azure --set secrets-store-csi-driver.enableSecretRotation=true --namespace csi --create-namespace
```

#### Register certificate with Geneva Metrics

With the certificate configuration done on the agent side, we will now let Geneva Metrics (MDM) know about the certificate. To do this follow the steps at [Authorize the new certificate in your Geneva Metrics account](~/collect/authentication/keyvaultmetricsauthorize.md)  
  
> If you want to have multiple metrics accounts set up for ingesting Prometheus metrics, you'd need to repeat the steps above for each account.  
  
--------------------------------------

In this step, you set up authentication for metrics collection from your Kubernetes cluster into your Geneva Metrics account.

Next, we will set up an agent that will collect metrics from your Kubernetes cluster. [Deploy agent to Kubernetes cluster for metrics collection](~/metrics/prometheus/PromMDMTutorial2DeployAgentHELM.md)
