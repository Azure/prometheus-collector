# This is the file that is supplied to powershell dsc so it knows what to do

Configuration Setup
{
    Import-DscResource -ModuleName 'PSDesiredStateConfiguration'
    Node localhost
    {
        Script Install
        {
            GetScript = { return; }            
            TestScript = { return $false; }
            SetScript = 
            {
                $ErrorActionPreference = 'Stop';

                function Install-Windows-Exporter
                {
                    msiexec /i C:\PROGRA~1\WindowsPowerShell\Modules\dscResources\windows_exporter-0.16.0-amd64.msi ENABLED_COLLECTORS=[defaults],process,container,tcp,os,memory /quiet
                }
                
                Install-Windows-Exporter;
            }
        }
    }
}