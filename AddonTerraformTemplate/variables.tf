variable "agent_count" {
  default = 3
}
variable "cluster_name" {
  default = "k8stest"
}

variable "metric_labels_allowlist" {
  default = null
}

variable "metric_annotations_allowlist" {
  default = null
}

variable "dns_prefix" {
  default = "k8stest"
}

variable "grafana_name" {
  default = "grafana-prometheus"
}

variable "grafana_sku" {
  default = "Standard"
}

variable "grafana_location" {
  default = "eastus"
}

variable "monitor_workspace_name" {
  default = "amwtest"
}

variable "resource_group_location" {
  default     = "eastus"
  description = "Location of the resource group."
}
