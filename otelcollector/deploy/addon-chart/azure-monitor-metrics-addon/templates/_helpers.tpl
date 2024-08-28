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

maxReplicasFromHelper: 8
minReplicasFromHelper: 2

{{/* If the current HPA already exists, set the HPA values to the current 
     HPA spec to preserve those values. */}}

{{- $currentHPA := lookup "autoscaling/v2" "HorizontalPodAutoscaler" "kube-system" $hpaName }}
{{- if and $currentHPA $currentHPA.spec }}
{{- $minReplicasFromCurrentSpec := 2 -}}
{{- $maxReplicasFromCurrentSpec := 8 -}}

  {{- if and ($currentHPA.spec.minReplicas) (gt (int $currentHPA.spec.minReplicas) 0) }}
{{- $minReplicasFromCurrentSpec = $currentHPA.spec.minReplicas -}}
    {{- if ge (int $minReplicasFromCurrentSpec) $autoscaleMin -}}
minReplicasFromHelper: {{ $minReplicasFromCurrentSpec }}
    {{- end }}
  {{- end }}

  {{- if and ($currentHPA.spec.maxReplicas) (gt (int $currentHPA.spec.maxReplicas) 0) }}
{{- $maxReplicasFromCurrentSpec = $currentHPA.spec.maxReplicas -}}
    {{- if le (int $maxReplicasFromCurrentSpec) $autoscaleMax -}}  
maxReplicasFromHelper: {{ $maxReplicasFromCurrentSpec }}
    {{- end }}
  {{- end }}

{{- end }}

{{- end }} 

