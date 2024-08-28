{{/* HPA merge. */}}
{{/* 
  1. Set the default HPA values for minReplicas, maxReplicas, and metrics. 
  2. If the current HPA already exists, override the default HPA values to the current values.
*/}}
{{ define "merge-discovery-hpa" }}

{{- $hpaName := "ama-metrics-hpa" }}
{{- $deploymentName := "ama-metrics" -}}

hpaName: {{ $hpaName }}
deploymentName: {{ $deploymentName }}

{{/* Set the default HPA values for minReplicas, maxReplicas, and metrics.  */}}

{{- $autoscaleMin := 2 -}}
{{- $autoscaleMax := 8 -}}
{{- $minReplicasValue := 2 -}}

maxReplicasFromHelper: 8
minReplicasFromHelper: 2
{{/* targetAverageValue: 10Gi */}}
{{/* 
metrics:
- type: ContainerResource
  containerResource:
    name: memory
    container: prometheus-collector
    target:
      averageValue: 10Gi
      type: AverageValue
*/}}

{{/* If the current HPA already exists, set the HPA values to the current 
     HPA spec to preserve those values. */}}

{{- $currentHPA := lookup "autoscaling/v2" "HorizontalPodAutoscaler" "kube-system" $hpaName }}
{{- if and $currentHPA $currentHPA.spec }}
{{- $minReplicasFromCurrentSpec := 2 -}}
{{- $maxReplicasFromCurrentSpec := 8 -}}
 {{/* $currentHPA.spec.minReplicas */}}
  {{- if and ($currentHPA.spec.minReplicas) (gt (int $currentHPA.spec.minReplicas) 0) }}
{{- $minReplicasFromCurrentSpec = $currentHPA.spec.minReplicas -}}
  {{- end }}
  {{- if and ($currentHPA.spec.minReplicas) (gt (int $currentHPA.spec.maxReplicas) 0) }}
{{- $maxReplicasFromCurrentSpec = $currentHPA.spec.maxReplicas -}}
  {{- end }}


  {{- if and (ge (int $minReplicasFromCurrentSpec) $autoscaleMin) (le (int $minReplicasFromCurrentSpec) $maxReplicasFromCurrentSpec) -}}
minReplicasFromHelper: {{ $minReplicasFromCurrentSpec }}
{{- $minReplicasValue = $minReplicasFromCurrentSpec -}}
  {{- end }}

  {{- if and (le (int $maxReplicasFromCurrentSpec) $autoscaleMax) (ge (int $maxReplicasFromCurrentSpec) $minReplicasValue) -}}  
 {{/* maxReplicasFromHelper: {{ $maxReplicasFromCurrentSpec }} */}}
  {{- end }}

{{/* {{- if and $currentHPA.spec $currentHPA.spec.metrics $currentHPA.spec.metrics.containerResource $currentHPA.spec.metrics.containerResource.target $currentHPA.spec.metrics.containerResource.target.averageValue }}
    {{- $validMemoryValue := regexMatch "(\\d+)Gi$" $currentHPA.spec.metrics.containerResource.target.averageValue -}}
     {{- if $validMemoryValue -}}
targetAverageValue: {{ $currentHPA.spec.metrics.containerResource.target.averageValue }}
     {{- end }}
  {{- end }}
{{- end }}
 {{- end }} */}}


{{/*  {{- if and $currentHPA.spec $currentHPA.spec.metrics -}}
    {{- range $key, $value := $currentHPA.spec.metrics }} 
    {{- $containerResource := $value.containerResource }}
      {{- if and $containerResource $containerResource.target $containerResource.target.averageValue -}}
        {{- $validMemoryValue := regexMatch "(\\d+)Gi$" $containerResource.target.averageValue -}}
        {{- if $validMemoryValue -}}
targetAverageValue: {{ $containerResource.target.averageValue }}
        {{- end }}
      {{- end }}
    {{- end }}
  {{- end }} */}}
  
{{- end }}

{{- end }} 

