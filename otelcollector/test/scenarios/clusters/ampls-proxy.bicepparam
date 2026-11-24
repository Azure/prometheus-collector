using 'aks.bicep'
import { linuxPools } from 'pools.bicep'

param runName = readEnvironmentVariable('SCENARIO_RUN_NAME')
param scenarioId = 'ampls-proxy'
param location = readEnvironmentVariable('SCENARIO_LOCATION')
param kubernetesVersion = readEnvironmentVariable('SCENARIO_KUBERNETES_VERSION')
param owner = readEnvironmentVariable('SCENARIO_OWNER')
param nodePools = linuxPools
param privateApi = true
param privateMonitoring = true
param proxyEnabled = true
param proxyAdminSshPublicKey = readEnvironmentVariable('SCENARIO_PROXY_SSH_PUBLIC_KEY')
param proxyVmSize = 'Standard_D2s_v4'
param proxyVmCount = 1
