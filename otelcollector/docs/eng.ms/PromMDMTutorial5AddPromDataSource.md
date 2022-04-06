> [!Note]
> Prometheus metrics in MDM is still in active development and is offered as a Private Preview. You can join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly in Public Preview.

# Prometheus data source


In the Prometheus Private Preview we use Azure Managed Grafana as our user interface to interact with the Prometheus metrics you have collected. If you have not already done so, please [sign up for the Managed Grafana Private Preview](../../dashboards/grafana/Tutorial0SetUpGrafanaAMG.md).

For Grafana to access your Prometheus metrics it needs to know where the data is and be able to access it with the right authentication. This is accomplished by a data source. Prometheus data source is supported natively in Grafana. We need to have this be configured to pull metrics from your MDM account instead of a Prometheus tsdb.
  
Follow the instructions below to setup your Prometheus data source.  
    
* Go to the cog icon in the Grafana side menu. If the side menu is not visible click the Grafana icon in the upper left corner.  

* Move your cursor over the cog icon to see the configuration options.  
![Add datasource2](~/metrics/images/prometheus/AMGAddDatasource2.png)  

* Click on **Data Sources**. The data sources page opens up, showing a list of all currently configured data sources for the Grafana instance.

![Add datasource4](~/metrics/images/prometheus/oob-data-sources.png)  
Notice that Azure Managed Grafana includes a Prometheus datasource pre-configured to access Container Insights demo data and to provide a working example of the required configuration for Azure Prometheus. Feel free to view the config before adding your own or editing this existing datasource.
* Click **Add data source**  

* Here are the steps for adding your own. Search for the Prometheus Datasource. Hover over it and click **Select**.  
![Add datasource4](~/metrics/images/prometheus/AMGAddDatasource5.png)  

* Configure data source to pull from your MDM account

In the data source configuration fill in the fields per guidance below.  

- Set **Name** to what you want your data source to be called. Also you can make this the 'default' for the Grafana cluster by enabling the Default toggle By default the Azure Monitor-Prometheus data source is configured to pull metrics from a sample Geneva Metrics (MDM) account.  
> [!Note]
> Please make sure to use a new name for the data source. Reusing previous data source names will result in errors during saving the datasource.
- Under HTTP section, populate **URL** field with the query endpoint that will be used by Grafana to pull metrics from the MDM tsdb. The URL should be:
    - if you are just starting, you know your MDM account name, and don't worry about query performance: _https://az-ncus.prod.prometheusmetrics.trafficmanager.net_
    - Else, refer to _Query endpoint_ section at [this page](ConsumePromWebApi.md) to figure out your query endpoint.

- If you are using Azure Managed Grafana Private preview version, you have two options to enable authentication
    - **Managed Identity**: This is the easiest way.
    - System Managed Identity is enabled by default on the Grafana resource.
    - Enable **Azure Authentication** during above data source setup, select “Managed Identity” from drop down and provide **AAD Resource Id** as _https://prometheus.monitor.azure.com_
   
   ![Add AMGAddDatasourcePP3](~/metrics/images/prometheus/AMGAddDatasourcePP3.png)

    - **AAD App registration**: This is the second option
    - Create an AAD App in MSFT CORP tenant (72f988bf-86f1-41af-91ab-2d7cd011db47) or AME tenant (33e01921-4d64-4f8c-a055-5bdaffd5e33d).
    - Enable “Azure Authentication” during data source setup, select “App Registration” from drop down and provide following details -
      - Directory (tenant) ID: Go to AAD APP in portal, overview page and copy the tenant Id. 
      - Application (client) ID: Go to AAD APP in portal, overview page and copy the Application (client) ID.
      - Client Secret: Go to AAD APP in portal, create a new secret and copy it. You will get 1 time opportunity to copy it after creation.
      - AAD Resource Id: _https://prometheus.monitor.azure.com_
    
     ![Add AMGAddDatasourcePP4](~/metrics/images/prometheus/AMGAddDatasourcePP4.png)
  
- Finally, you need to let the data source know your specific MDM account. If you are using MAC based query endpoint, this step is not required and query service ignores this header if passed with MAC based query URL. Otherwise, pass via a custom header during data source setup. In Custom HTTP headers section, click **+ Add header**  
    - For **Header** enter 'X-Ms-Mdm-Account-Name'.
    - For **Value** enter your actual MDM metrics account

Note: Few customers have already used old header name **mdmAccountName** for this which we will continue to support. The new new name is more aligned towards Microsoft naming convention.

![Add datasource7](~/metrics/images/prometheus/AMGAddDatasource7.png)  

* Click **Save and test** to validate the data source. You are ready once you see a 'Data source is working' confirmation.  

--------------------------------------

In this step you set up Grafana to access Prometheus metrics from your MDM metrics account. Go to this [link](https://grafana.com/docs/grafana/v7.5/datasources/prometheus/) for more information on Prometheus for Grafana  

Next, you will look at how to use the several [built-in dashboards](~/metrics/Prometheus/PromMDMTutorial6ReuseExistingDashboard.md) that are available for you out of the box.

Lastly, please refer to [this](ConsumePromWebApi.md) page for limitations on case sensitivity, time range etc. and other details.