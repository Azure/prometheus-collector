If you are deploying a new AKS cluster using Terraform with managed Prometheus addon enabled, follow the steps below.

1. Please download all files under AddonTerraformTemplate.
2. Run `terraform init -upgrade` to initialize the Terraform deployment.
3. Run `terraform plan -out main.tfplan` to initialize the Terraform deployment.
3. Run `terraform apply main.tfplan` to apply the execution plan to your cloud infrastructure.

Assign the Monitoring Data Reader role to the Grafana MSI on the Azure Monitor workspace resource so that it can read data for displaying the charts. Use the following instructions.

1. On the Overview page for the Azure Managed Grafana instance in the Azure portal, select JSON view.

2. Copy the value of the principalId field for the SystemAssigned identity.

```
"identity": {
        "principalId": "00000000-0000-0000-0000-000000000000",
        "tenantId": "00000000-0000-0000-0000-000000000000",
        "type": "SystemAssigned"
    },
```

3. On the Access control (IAM) page for the Azure Monitor Workspace in the Azure portal, select Add > Add role assignment.

4. Select Monitoring Data Reader.

5. Select Managed identity > Select members.

6. Select the system-assigned managed identity with the principalId from the Grafana resource.

7. Choose Select > Review+assign.
