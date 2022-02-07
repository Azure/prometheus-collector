> [!Note]
> Prometheus metrics in MDM is still in active development and is offered as a Private Preview. You can join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly in Public Preview.

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

### Install Azure KeyVault driver in your cluster

To be able to mount secrets and certificates as storage volumes, you will need to install the csi driver & secrets store provider for Azure KeyVault in your cluster.
For this we will installl the AKS addon by running the following commands.

```shell
az aks enable-addons --addons azure-keyvault-secrets-provider --name <AKS cluster name> --resource-group <resource group name> --enable-secret-rotation
```

See the [documentation](https://docs.microsoft.com/en-us/azure/aks/csi-secrets-store-driver) for more information.

### Set up permissions for agent to pull certificate from KeyVault

Next, lets ensure the agent and Azure Key Vault driver have the right authorization to pull the certificate from KeyVault. To do this we will need to use either a service principal or a managed identity.

#### Service Principal

To create a service principal using the Azure Portal:

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

#### Managed Identity

To use a User-Assigned Managed Identity for Key Vault access:
* Get the client ID of the User-Assigned Managed Identity you would like to use. You can show the client ID of the identity of the Azure Key Vault Secrets Provider by running:

  ```shell
  az aks show -g <resource-group> -n <cluster-name> --query addonProfiles.azureKeyvaultSecretsProvider.identity.clientId -o tsv
  ```
* Grant permissions:

  ```shell
  # set policy to access certs in your key vault
  az keyvault set-policy -n <keyvault-name> --certificate-permissions get --spn <identity-client-id>
  ```
* Save the identity client ID, as this is needed in a subsequent step.

To use a System-Assigned Managed Idenity:
* Enable System-Assigned Managed Identity by following [these instructions](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/qs-configure-cli-windows-vm#enable-system-assigned-managed-identity-on-an-existing-azure-vm).
* Get the principal ID of the System-Assigned Managed Identity:
  ```shell
  az vmss identity show -g <resource group>  -n <vmss scalset name> -o yaml
  az vm identity show -g <resource group> -n <vm name> -o yaml
  ```
* Grant permissions:
  ```shell
  # set policy to access certs in your key vault
  az keyvault set-policy -n <keyvault-name> --certificate-permissions get --spn <identity-principal-id>
  ```

See more details about configuring identity access [here](https://docs.microsoft.com/en-us/azure/aks/csi-secrets-store-identity-access).

### Register certificate with Geneva Metrics

With the certificate configuration done on the agent side, we will now let Geneva Metrics (MDM) know about the certificate. To do this follow the steps at [Authorize the new certificate in your Geneva Metrics account](~/collect/authentication/keyvaultmetricsauthorize.md)  
  
> If you want to have multiple metrics accounts set up for ingesting Prometheus metrics, you'd need to repeat the steps above for each account.  
  
--------------------------------------

In this step, you set up authentication for metrics collection from your Kubernetes cluster into your Geneva Metrics account.

Next, we will set up an agent that will collect metrics from your Kubernetes cluster. [Deploy agent to Kubernetes cluster for metrics collection](~/metrics/prometheus/PromMDMTutorial2DeployAgentHELM.md)
