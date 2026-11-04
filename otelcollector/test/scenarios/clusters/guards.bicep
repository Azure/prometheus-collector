@allowed([true])
param windowsNeedsAzure bool
@allowed([true])
param windowsNeedsPassword bool
@allowed([true])
param windows2022Version bool
@allowed([true])
param windows2025Version bool
@allowed([true])
param acnsNeedsAzure bool
@allowed([true])
param proxyNeedsSshKey bool
@allowed([true])
param proxyCaNeedsProxy bool
@allowed([true])
param proxyNoProxyIsScoped bool
@allowed([true])
param validPoolPlan bool
@allowed([true])
param classicNeedsAzure bool
@allowed([true])
param windowsFipsVmSupported bool

output validated bool = windowsNeedsAzure && windowsNeedsPassword && windows2022Version && windows2025Version && acnsNeedsAzure && proxyNeedsSshKey && proxyCaNeedsProxy && proxyNoProxyIsScoped && validPoolPlan && classicNeedsAzure && windowsFipsVmSupported
