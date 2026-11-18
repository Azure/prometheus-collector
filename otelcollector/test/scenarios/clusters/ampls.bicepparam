using 'aks.bicep'
import { defaultPools } from 'pools.bicep'

param runName = readEnvironmentVariable('SCENARIO_RUN_NAME')
param scenarioId = 'ampls'
param location = readEnvironmentVariable('SCENARIO_LOCATION')
param kubernetesVersion = readEnvironmentVariable('SCENARIO_KUBERNETES_VERSION')
param owner = readEnvironmentVariable('SCENARIO_OWNER')
param windowsAdminPassword = readEnvironmentVariable('SCENARIO_WINDOWS_PASSWORD')
param privateMonitoring = true
param nodePools = defaultPools
