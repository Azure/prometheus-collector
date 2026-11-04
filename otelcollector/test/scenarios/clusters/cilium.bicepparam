using 'aks.bicep'
import { linuxPools } from 'pools.bicep'

param runName = readEnvironmentVariable('SCENARIO_RUN_NAME')
param scenarioId = 'cilium'
param location = readEnvironmentVariable('SCENARIO_LOCATION')
param kubernetesVersion = readEnvironmentVariable('SCENARIO_KUBERNETES_VERSION')
param owner = readEnvironmentVariable('SCENARIO_OWNER')
param expiresAt = readEnvironmentVariable('SCENARIO_EXPIRES_AT')
param nodePools = linuxPools
param dataplane = 'cilium'
