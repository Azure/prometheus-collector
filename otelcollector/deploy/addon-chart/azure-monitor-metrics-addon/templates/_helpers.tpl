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

{{- $autoscaleMin := 2 }}
{{- $autoscaleMax := 8 -}}

maxReplicas: 8
minReplicas: 2

{{/* If the current HPA already exists, set the HPA values to the current 
     HPA spec to preserve those values. */}}

{{- $currentHPA := lookup "autoscaling/v2" "HorizontalPodAutoscaler" "kube-system" $hpaName }}
{{- if $currentHPA }}
  {{- if ge (int $currentHPA.spec.minReplicas) $autoscaleMin -}}
minReplicas: {{ $currentHPA.spec.minReplicas }}
  {{- end }}
  {{- if le (int $currentHPA.spec.minReplicas) $autoscaleMax -}}  
maxReplicas: {{ $currentHPA.spec.maxReplicas }}
  {{- end }}
{{- end }}

{{- end }}
