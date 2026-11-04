@export()
type nodePool = {
  @minLength(1)
  @maxLength(12)
  name: string
  osType: 'Linux' | 'Windows'
  osSKU: 'Ubuntu' | 'AzureLinux3' | 'Windows2022' | 'Windows2025'
  architecture: 'amd64' | 'arm64'
  mode: 'System' | 'User'
  vmSize: string
  fips: bool
  @minValue(1)
  count: int
  autoscaling: bool
  @minValue(1)
  minCount: int
  @minValue(1)
  maxCount: int
  maxSurge: '1'
}

@export()
var linuxPools = [
  {
    name: 'system'
    osType: 'Linux'
    osSKU: 'Ubuntu'
    architecture: 'amd64'
    mode: 'System'
    vmSize: 'Standard_D4s_v4'
    fips: false
    count: 1
    autoscaling: false
    minCount: 1
    maxCount: 1
    maxSurge: '1'
  }
]

var windowsPool = {
  name: 'win22'
  osType: 'Windows'
  osSKU: 'Windows2022'
  architecture: 'amd64'
  mode: 'User'
  vmSize: 'Standard_D4s_v4'
  fips: false
  count: 1
  autoscaling: false
  minCount: 1
  maxCount: 1
  maxSurge: '1'
}

@export()
var defaultPools = concat(linuxPools, [windowsPool])

var extraLinuxPools = [
  { name: 'azlinux', osSKU: 'AzureLinux3', architecture: 'amd64', vmSize: 'Standard_D4s_v4', fips: false }
  { name: 'ubarm', osSKU: 'Ubuntu', architecture: 'arm64', vmSize: 'Standard_D4ps_v5', fips: false }
  { name: 'azarm', osSKU: 'AzureLinux3', architecture: 'arm64', vmSize: 'Standard_D4ps_v5', fips: false }
  { name: 'ubfips', osSKU: 'Ubuntu', architecture: 'amd64', vmSize: 'Standard_D4s_v4', fips: true }
  { name: 'azfips', osSKU: 'AzureLinux3', architecture: 'amd64', vmSize: 'Standard_D4s_v4', fips: true }
  { name: 'azarmfips', osSKU: 'AzureLinux3', architecture: 'arm64', vmSize: 'Standard_D4ps_v5', fips: true }
]

var normalizedExtraLinuxPools = [for pool in extraLinuxPools: union(pool, {
  osType: 'Linux'
  mode: 'User'
  count: 1
  autoscaling: false
  minCount: 1
  maxCount: 1
  maxSurge: '1'
})]

@export()
var mixedPools = concat(defaultPools, normalizedExtraLinuxPools, [
  union(windowsPool, { name: 'winfip', fips: true })
  union(windowsPool, { name: 'win25', osSKU: 'Windows2025', fips: true })
])

@export()
var autoscalerPools = map(defaultPools, pool => union(pool, { autoscaling: true, maxCount: 3 }))
