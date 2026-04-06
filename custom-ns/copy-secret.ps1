# Copy aad-msi-auth-token secret from kube-system to target namespace
param(
    [string]$TargetNamespace = "ama-metrics-zane-test"
)

Write-Host "Target namespace: $TargetNamespace"
Write-Host ""
$Reply = Read-Host "Proceed? (yes/no)"
if ($Reply -ne "yes") {
    Write-Host "Cancelled."
    exit 0
}

# Delete existing secret if present
$existing = kubectl get secret aad-msi-auth-token -n $TargetNamespace 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "Secret aad-msi-auth-token already exists in $TargetNamespace"
    $Reply = Read-Host "Delete and replace? (yes/no)"
    if ($Reply -ne "yes") {
        Write-Host "Cancelled."
        exit 0
    }
    kubectl delete secret aad-msi-auth-token -n $TargetNamespace
}

Write-Host "Copying aad-msi-auth-token from kube-system to $TargetNamespace..."
kubectl get secret aad-msi-auth-token -n kube-system -o yaml |
    ForEach-Object { $_ -replace 'namespace: kube-system', "namespace: $TargetNamespace" } |
    kubectl apply -f -

Write-Host "Done."
