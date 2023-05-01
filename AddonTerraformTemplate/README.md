If you are deploying a new AKS cluster using Terraform with managed Prometheus addon enabled, follow the steps below.

1. Please download all files under AddonTerraformTemplate.
2. Run `terraform init -upgrade` to initialize the Terraform deployment.
3. Run `terraform plan -out main.tfplan` to initialize the Terraform deployment.
3. Run `terraform apply main.tfplan` to apply the execution plan to your cloud infrastructure.


Note: Pass the variables for `annotations_allowed` and `labels_allowed` keys only when those values exist. These are optional blocks.

**NOTE**
- Please edit the main.tf file appropriately before running the terraform template
- Please add in any existing azure_monitor_workspace_integrations values to the grafana resource before running the template otherwise the older values will get deleted and replaced with what is there in the template at the time of deployment
- Users with 'User Access Administrator' role in the subscription  of the AKS cluster can be able to enable 'Monitoring Data Reader' role directly by deploying the template.
- Please edit the grafanaSku parameter if you are using a non standard SKU.
- Please run this template in the Grafana Resources RG.
