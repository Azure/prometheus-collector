variable "agent_count" {
  default = 3
}
variable "cluster_name" {
  default = "k8stest"
}

variable "dns_prefix" {
  default = "k8stest"
}

variable "monitor_workspace_id" {
  default = "/subscriptions/{sub_id}/resourceGroups/{rg_name}/providers/microsoft.monitor/accounts/{amw_name}"
}

variable "resource_group_location" {
  default     = "eastus"
  description = "Location of the resource group."
}

variable "resource_group_name_prefix" {
  default     = "rg"
  description = "Prefix of the resource group name that's combined with a random ID so name is unique in your Azure subscription."
}

