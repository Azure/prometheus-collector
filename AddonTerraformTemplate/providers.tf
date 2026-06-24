terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~>3.0"
    }
    azapi = {
      source  = "Azure/azapi"
      version = "~>1.15"
    }
  }
}

provider "azurerm" {
  features {}
}

provider "azapi" {
}
