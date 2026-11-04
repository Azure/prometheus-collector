using 'aks.bicep'
import { defaultPools } from 'pools.bicep'

param runName = readEnvironmentVariable('SCENARIO_RUN_NAME')
param scenarioId = 'ampls-acns'
param location = readEnvironmentVariable('SCENARIO_LOCATION')
param kubernetesVersion = readEnvironmentVariable('SCENARIO_KUBERNETES_VERSION')
param owner = readEnvironmentVariable('SCENARIO_OWNER')
param expiresAt = readEnvironmentVariable('SCENARIO_EXPIRES_AT')
param windowsAdminPassword = readEnvironmentVariable('SCENARIO_WINDOWS_PASSWORD')
param privateMonitoring = true
param acns = true
param nodePools = defaultPools
