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
                --settings '{\"wmfVersion\":\"latest\", \"configuration\":{\"url\":\"https://github.com/Azure/prometheus-collector/releases/download/windows-exporter-setup/aksSetup.zip\", \"script\":\"aksSetup.ps1\", \"function\":\"Setup\"}}' `
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

Deploy-PrometheusWindowsExporter -subscription "ce4d1293-71c0-4c72-bc55-133553ee9e50" -resourceGroup "MC_kaveeshWinExporter_kaveeshWinExporter_eastus";