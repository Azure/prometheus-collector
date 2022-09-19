## Default recommended Prometheus alert rules

By default, Azure monitor Managed Prometheus agent does not setup any alert rules on the Azure Monitor workspace for the monitored cluster.
We have an alert template with hand-picked alerts from Prometheus community that we recommend you to try, by manually importing the ARM template found [here](https://github.com/Azure/prometheus-collector/blob/main/GeneratedMonitoringArtifacts/Default/DefaultAlerts.json). Below are the alerts defined in this template. Source code for these mixin alerts can be found [here](https://github.com/Azure/prometheus-collector/tree/main/mixins)


1. KubeJobNotCompleted
2. KubeJobFailed
3. KubePodCrashLooping
4. KubePodNotReady
5. KubeDeploymentReplicasMismatch
6. KubeStatefulSetReplicasMismatch
7. KubeHpaReplicasMismatch
8. KubeHpaMaxedOut
9. KubeQuotaAlmostFull
10. KubeMemoryQuotaOvercommit
11. KubeCPUQuotaOvercommit
12. KubeVersionMismatch
13. KubeNodeNotReady
14. KubeNodeReadinessFlapping
15. KubeletTooManyPods
16. KubeNodeUnreachable

Current users of [Container Insights Log based recommended alerts](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/container-insights-metric-alerts) can also try the equivalent Prometheus alerts in [this](https://github.com/Azure/prometheus-collector/blob/main/mixins/kubernetes/rules/recording_and_alerting_rules/templates/ci_recommended_alerts.json) ARM template .
