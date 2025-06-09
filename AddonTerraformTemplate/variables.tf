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

variable "grafana_version" {
  default = "10"
}

variable "is_private_cluster" {
  default = "false"
}

variable "monitor_workspace_name" {
  default = "amwtest"
}

variable "amw_region" {
  default = "northeurope"
  description = "Location of the Azure Monitor Workspace"
}

variable "cluster_region" {
  default = "eastus"
  description = "Location of the Azure Kubernetes Cluster"
}

variable "resource_group_location" {
  default     = "eastus"
  description = "Location of the resource group."
}

variable "enable_windows_recording_rules" {
  type    = bool
  default = false
  description = "Enable UX recording rules for Windows (default: false)"
}
