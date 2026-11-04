using 'aks.bicep'
import { autoscalerPools } from 'pools.bicep'

param runName = readEnvironmentVariable('SCENARIO_RUN_NAME')
param scenarioId = 'autoscaler'
param location = readEnvironmentVariable('SCENARIO_LOCATION')
param kubernetesVersion = readEnvironmentVariable('SCENARIO_KUBERNETES_VERSION')
param owner = readEnvironmentVariable('SCENARIO_OWNER')
param expiresAt = readEnvironmentVariable('SCENARIO_EXPIRES_AT')
param windowsAdminPassword = readEnvironmentVariable('SCENARIO_WINDOWS_PASSWORD')
param nodePools = autoscalerPools
