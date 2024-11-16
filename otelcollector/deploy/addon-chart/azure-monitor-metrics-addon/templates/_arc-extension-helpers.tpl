{{ define "arc-extension-settings" }}

# true if Arc Extension is specifically set to true
{{ $isArcExtension := eq .Values.AzureMonitorMetrics.ArcExtension true }}
isArcExtension: {{ $isArcExtension }}
resourceId: {{.Values.Azure.Cluster.ResourceId }}
region: {{ .Values.Azure.Cluster.Region }}

# If our override CloudEnvironment value is set, use that. Otherwise, use inherited Arc cloud helm value 
cloudEnvironment: {{ default (lower .Values.Azure.Cluster.Cloud) (lower .Values.CloudEnvironment) }}
# If our override ClusterDistribution value is set, use that. Otherwise, use inherited Arc cluster helm value
distribution: {{ default (lower .Values.Azure.Cluster.Distribution) (lower .Values.ClusterDistribution) }}
# true if Arc Extension is enabled and inherited Arc helm values isProxyEnabled is true
isProxyEnabled: {{ and ($isArcExtension) (.Values.Azure.proxySettings.isProxyEnabled) }}

operatorEnabled: true
{{- if $isArcExtension }}
    {{- if or (ne .Values.AzureMonitorMetrics.ArcEnableOperator true) (ne .Values.AzureMonitorMetrics.TargetAllocatorEnabled true) }}
        operatorEnabled: false
    {{- end }}
{{- end }}

hpaEnabled: true
{{- if or ($isArcExtension) (ne .Values.AzureMonitorMetrics.CollectorHPAEnabled true) }}
    hpaEnabled: false
{{- end }}

# Get node-exporter values
nodeExporterTargetPort: {{ index .Values "prometheus-node-exporter" "service" "targetPort" }}
nodeExporterVersion: "1.8.2"

mountMarinerCerts: {{ eq .Values.MountCATrustAnchorsDirectory true }} 
mountUbuntuCerts: {{ eq .Values.MountUbuntuCACertDirectory true }}
{{- if $isArcExtension }}
    # Keep backwards compatible for aks_edge either through our override ClusterDistribution value or inherited Arc cluster helm value
    {{- if or (hasPrefix "aks_edge" .Values.ClusterDistribution) (or (eq .Values.Azure.Cluster.Distribution "aks_edge_k3s") (eq .Values.Azure.Cluster.Distribution "aks_edge_k8s")) }}
        mountUbuntuCerts: false
    {{- end }}
    {{- if (eq .Values.MountUbuntuCACertDirectory false) }}
        mountUbuntuCerts: false
    {{- end }}
    {{- if (eq .Values.MountCATrustAnchorsDirectory false) }}
        mountMarinerCerts: false
    {{- end }}
{{- end }}

{{- end }} 