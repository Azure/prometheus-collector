AZURE_PUBLIC_CLOUD_ENDPOINTS = {
    "activeDirectory": "https://login.microsoftonline.com/",
    "activeDirectoryDataLakeResourceId": "https://datalake.azure.net/",
    "activeDirectoryGraphResourceId": "https://graph.windows.net/",
    "activeDirectoryResourceId": "https://management.core.windows.net/",
    "appInsights": "https://api.applicationinsights.io",
    "appInsightsTelemetryChannel": "https://dc.applicationinsights.azure.com/v2/track",
    "batchResourceId": "https://batch.core.windows.net/",
    "gallery": "https://gallery.azure.com/",
    "logAnalytics": "https://api.loganalytics.io",
    "management": "https://management.core.windows.net/",
    "mediaResourceId": "https://rest.media.azure.net",
    "microsoftGraphResourceId": "https://graph.microsoft.com/",
    "ossrdbmsResourceId": "https://ossrdbms-aad.database.windows.net",
    "resourceManager": "https://management.azure.com/",
    "sqlManagement": "https://management.core.windows.net:8443/",
    "vmImageAliasDoc": "https://raw.githubusercontent.com/Azure/azure-rest-api-specs/master/arm-compute/quickstart-templates/aliases.json"
}

AZURE_DOGFOOD_ENDPOINTS = {
    "activeDirectory": "https://login.windows-ppe.net/",
    "activeDirectoryDataLakeResourceId": None,
    "activeDirectoryGraphResourceId": "https://graph.ppe.windows.net/",
    "activeDirectoryResourceId": "https://management.core.windows.net/",
    "appInsights": None,
    "appInsightsTelemetryChannel": None,
    "batchResourceId": None,
    "gallery": "https://df.gallery.azure-test.net/",
    "logAnalytics": None,
    "management": "https://management-preview.core.windows-int.net/",
    "mediaResourceId": None,
    "microsoftGraphResourceId": None,
    "ossrdbmsResourceId": None,
    "resourceManager": "https://api-dogfood.resources.windows-int.net/",
    "sqlManagement": None,
    "vmImageAliasDoc": None
}

AZURE_CLOUD_DICT = {"AZURE_PUBLIC_CLOUD" : AZURE_PUBLIC_CLOUD_ENDPOINTS, "AZURE_DOGFOOD": AZURE_DOGFOOD_ENDPOINTS}

TIMEOUT = 300

# ama-metrics main container name
AMA_LOGS_MAIN_CONTAINER_NAME = 'ama-metrics'

# WAIT TIME BEFORE READING THE AGENT LOGS
AGENT_WAIT_TIME_SECS = "180"
# Azure Monitor for Container Extension related
AGENT_RESOURCES_NAMESPACE = 'kube-system'
AGENT_DEPLOYMENT_NAME = 'ama-metrics'
AGENT_DAEMONSET_NAME = 'ama-metrics-node'

AGENT_DEPLOYMENT_PODS_LABEL_SELECTOR = 'rsName=ama-metrics'
AGENT_DAEMON_SET_PODS_LABEL_SELECTOR = 'dsName=ama-metrics-node'
AGENT_DAEMON_SET_PODS_LABEL_SELECTOR_NON_ARC = 'component=ama-metrics'

# override this through setting enviornment variable if the expected restart count is > 0 for example applying configmap
AGENT_POD_EXPECTED_RESTART_COUNT = 0
