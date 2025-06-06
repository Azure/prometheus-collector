{{/* HPA merge. */}}
{{/* 
  1. Set the default HPA values for minReplicas, maxReplicas, and metrics. 
  2. If the current HPA already exists, override the default HPA values to the current values.
*/}}
{{ define "ama-metrics-merge-custom-hpa" }}

{{/* Set the default HPA values for minReplicas, maxReplicas, and metrics.  */}}
{{- $amaMetricsHpaName := "ama-metrics-hpa" }}
{{- $amaMetricsAutoscaleMin := 2 -}}
{{- $amaMetricsAutoscaleMax := 24 -}}
{{- $amaMetricsAutoscaleMaxPrevious := 12 -}}


amaMetricsMinReplicasFromHelper: 2
amaMetricsMaxReplicasFromHelper: 24

{{/* If the current HPA already exists, set the HPA values to the current 
     HPA spec to preserve those values. */}}

{{- $amaMetricsCurrentHPA := lookup "autoscaling/v2" "HorizontalPodAutoscaler" "kube-system" $amaMetricsHpaName }}
{{- if and $amaMetricsCurrentHPA $amaMetricsCurrentHPA.spec }}
{{- $amaMetricsMinReplicasFromCurrentSpec := $amaMetricsCurrentHPA.spec.minReplicas -}}
{{- $amaMetricsMaxReplicasFromCurrentSpec := $amaMetricsCurrentHPA.spec.maxReplicas -}}

  {{- if and ($amaMetricsMinReplicasFromCurrentSpec) (gt (int $amaMetricsMinReplicasFromCurrentSpec) 0) }}
    {{- if ge (int $amaMetricsMinReplicasFromCurrentSpec) $amaMetricsAutoscaleMin }}
amaMetricsMinReplicasFromHelper: {{ $amaMetricsMinReplicasFromCurrentSpec }}
    {{- end }}
  {{- end }}

  {{- if and ($amaMetricsMaxReplicasFromCurrentSpec) (gt (int $amaMetricsMaxReplicasFromCurrentSpec) 0) }}
{{/* Adding this check to make sure the previous default max is updated with the new update  */}}
    {{- if  and (le (int $amaMetricsMaxReplicasFromCurrentSpec) $amaMetricsAutoscaleMax) (ne (int $amaMetricsMaxReplicasFromCurrentSpec) $amaMetricsAutoscaleMaxPrevious) }}
amaMetricsMaxReplicasFromHelper: {{ $amaMetricsMaxReplicasFromCurrentSpec }}
    {{- end }}
  {{- end }}

{{- end }}

{{- end }} 

