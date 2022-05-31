function Deploy-PrometheusWindowsExporter([string]$subscription, [string]$resourceGroup)
{
    $ErrorActionPreference = "Stop";
    $PSDefaultParameterValues['*:ErrorAction']='Stop';

    Write-Output "";
    Write-Output "Checking $subscription/$resourceGroup for windows vmss that need powershell dsc installed";

    $scaleSets = az vmss list --subscription $subscription --resource-group $resourceGroup | ConvertFrom-Json;

    # grab all the windows vmss in the rg
    $vmssNamesToRun = $scaleSets.Where({
        $_.virtualMachineProfile.osProfile.linuxConfiguration -eq $null `
        -and ($_.virtualMachineProfile.extensionProfile.extensions.where({$_.name -eq "Microsoft.Powershell.DSC"}).Count -eq 0)}) | `
        % Name;

    if($vmssNamesToRun.Length -gt 0)
    {
        Write-Output "";
        Write-Output "Installing DSC on the following VMSS:";
        Write-Output $vmssNamesToRun;

        foreach($vmssName in $vmssNamesToRun)
        {
            Write-Output "";

            Write-Output "Installing DSC on vmss $vmssName...";
            $installResult = az vmss extension set `
                --extension-instance-name "Microsoft.Powershell.DSC" `
                --name "DSC" `
                --publisher "Microsoft.Powershell" `
                --version "2.80" `
                --subscription $subscription `
                --resource-group $resourceGroup `
                --vmss-name $vmssName `
                --provision-after-extensions "vmssCSE" `
                --settings '{\"wmfVersion\":\"latest\", \"configuration\":{\"url\":\"https://github.com/bragi92/helloWorld/raw/master/aksSetup.zip\", \"script\":\"aksSetup.ps1\", \"function\":\"Setup\"}}' `
                --force-update;
        
            Write-Output "Updating instances on vmss $vmssName...";
            $updateResult = az vmss update-instances `
                --subscription $subscription `
                --resource-group $resourceGroup `
                --name $vmssName `
                --instance-ids *; 

            Write-Output "DSC installation complete on vmss $vmssName";
        }
    }

    Write-Output "";
    Write-Output "All windows vmss have powershell dsc extension installed in $subscription/$resourceGroup";
}

# Dev clusters
Deploy-PrometheusWindowsExporter -subscription "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb" -resourceGroup "MC_ci-dev-aks-mac-eus-rg_ci-dev-aks-mac-eus_eastus";
Deploy-PrometheusWindowsExporter -subscription "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb" -resourceGroup "MC_ci-dev-aks-msi-eus2-rg_ci-dev-aks-msi-eus2_eastus2";
Deploy-PrometheusWindowsExporter -subscription "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb" -resourceGroup "MC_ci-dev-aks-wcus-rg_ci-dev-aks-wcus_westcentralus";

# Prod clusters
Deploy-PrometheusWindowsExporter -subscription "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb" -resourceGroup "MC_ci-prod-aks-eus-rg_ci-prod-aks-eus_eastus";
Deploy-PrometheusWindowsExporter -subscription "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb" -resourceGroup "MC_ci-prod-aks-mac-weu-rg_ci-prod-aks-mac-weu_westeurope";
Deploy-PrometheusWindowsExporter -subscription "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb" -resourceGroup "MC_ci-prod-aks-msi-eus2-rg_ci-prod-aks-msi-eus2_eastus2";

