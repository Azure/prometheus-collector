You can create the policy definition using a command like :

```az policy definition create --name "(Preview) Prometheus Metrics addon" --display-name "(Preview) Prometheus Metrics addon" --mode Indexed --metadata version=1.0.0 category=Kubernetes --rules .\AddonPolicyMetricsProfile.rules.json --params .\AddonPolicyMetricsProfile.parameters.json```

**NOTE**

- Please download all files under AddonPolicyTemplate folder before running the policy template.
- After creating the policy definition through the above command, go to Azure portal -> Policy -> Definitions and select the definition you just created.
- Click on 'Assign' and then go to the 'Parameters' tab and fill in the details. Then click 'Review + Create'.
- Now that the policy is assigned to the subscription, whenever you create a new cluster which does not have Prometheus enabled, the policy will run and deploy the resources. If you want to apply the policy to existing AKS cluster, create a 'Remediation task' for that resource after going to the 'Policy Assignment'.
- Now you should see metrics flowing in the existing linked Grafana resource(linked with the corresponding Azure Monitor Workspace).
- You can also create a new Managed Grafana resource from Azure portal and link it with the corresponding Azure Monitor Workspace from the 'Linked Grafana Workspaces' tab under Azure Monitor Workspace. Please assign the role 'Monitoring Data Reader' to the Grafana MSI on the Azure Monitor Workspace resource so that it can read data for displaying the charts.
