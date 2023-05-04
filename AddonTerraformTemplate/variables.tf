variable "agent_count" {
  default = 3
}
variable "cluster_name" {
  default = "k8stest"
}

variable "cluster_location" {
  default = "eastus"
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

variable "monitor_workspace_id" {
  default = "/subscriptions/{sub_id}/resourceGroups/{rg_name}/providers/microsoft.monitor/accounts/{amw_name}"
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
variable "monitor_workspace_location" {
  default = "eastus"
}

variable "resource_group_location" {
  default     = "eastus"
  description = "Location of the resource group."
}
