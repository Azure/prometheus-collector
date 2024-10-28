# Increasing Azure Monitor Workspace ingestion limits with ARM API update

## Introduction
Azure Monitor Workspaces or AMW are containers that store data collected by Azure Monitor managed service for Prometheus. An AMW instance has certain limits on how much data it can ingest. These limits are set by default, but they can be customized by the customer by creating a support ticket. For more details on these limits, see Azure Monitor service limits - Azure Monitor | Microsoft Learn.
We are excited to share that customers can now update the ingestion limits for their AMW instance using the Azure Resource Manager (ARM) API. Few additional details about this update:
•	Customers can request for an increase in limit from 1 Mn events/min or active TS to up to 2 Mn events/min or active TS with an API update through cli or through ARM update. For limits above 2 mn, customers will need to create a support ticket. In the next version of this API, we will work on adding support for increasing limits beyond 2 mn.
•	Customers can request an increase for an existing AMW instance. We are not supporting creation of AMW with increased limits. Creation of AMW will always apply the default limits. This is because we want to support increasing the limits based on certain heuristics/usage.
This document will show you how to use the ARM API to update the data ingestion limits of your Azure Monitor Workspaces. 
