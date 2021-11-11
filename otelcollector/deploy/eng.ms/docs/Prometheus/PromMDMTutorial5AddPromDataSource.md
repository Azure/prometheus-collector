> [!Note]
> Prometheus metrics in MDM is still in active development and is offered as a Private Preview. You can join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly in Public Preview.

# Prometheus data source


In the Prometheus Private Preview we use Azure Managed Grafana as our user interface to interact with the Prometheus metrics you have collected. If you have not already done so, please [sign up for the Managed Grafana Private Preview](../../dashboards/grafana/Tutorial0SetUpGrafanaAMG.md).

For Grafana to access your Prometheus metrics it needs to know where the data is and be able to access it with the right authentication. This is accomplished by a data source. Prometheus data source is supported natively in Grafana. We need to have this be configured to pull metrics from your MDM account instead of a Prometheus tsdb.  
  
If you are using the Azure Managed Workspace for Grafana preview, refer to [this doc](https://github.com/microsoft/azure-grafana-preview-doc/blob/main/ConfigureDataSources.md) to setup your Prometheus data source. For Dogfood users, follow the instructions below.  
    
* Go to the cog icon in the Grafana side menu. If the side menu is not visible click the Grafana icon in the upper left corner.  

* Move your cursor over the cog icon to see the configuration options.  
![Add datasource2](~/metrics/images/prometheus/AMGAddDatasource2.png)  

* Click on **Data Sources**. The data sources page opens up, showing a list of all currently configured data sources for the Grafana instance.

* Click **Add data source**  
![Add datasource4](~/metrics/images/prometheus/AMGAddDatasource4.png)  

* You will see the Prometheus data source listed (if it doesn't show up you can search for it). Hover over it and click **Select**.  
![Add datasource4](~/metrics/images/prometheus/AMGAddDatasource5.png)  

* Configure data source to pull from your MDM account

In the data source configuration fill in the fields per guidance below.  

- Set **Name** to what you want your data source to be called. Also you can make this the 'default' for the Grafana cluster by enabling the Default toggle By default the Azure Monitor-Prometheus data source is configured to pull metrics from a sample Geneva Metrics (MDM) account.  
> [!Note]
> Please make sure to use a new name for the data source. Reusing previous data source names will result in errors during saving the datasource.
- Under HTTP section, set **URL** to 'https://az-eus.prod.prometheusmetrics.trafficmanager.net' . This is the end point used by Grafana to pull metrics from the MDM tsdb.  

- If you are using Azure Managed Grafana Private preview version, you have two options to enable authentication
    - **Managed Identity**: This is the easiest way.
    - System Managed Identity is enabled by default on the Grafana resource.
    - Enable **Azure Authentication** during above data source setup, select “Managed Identity” from drop down and provide **AAD Resource Id** as https://management.azure.com
   
   ![Add AMGAddDatasourcePP3](~/metrics/images/prometheus/AMGAddDatasourcePP3.png)

    - **AAD App registration**: This is the second option
    - Create an AAD App in MSFT CORP tenant (72f988bf-86f1-41af-91ab-2d7cd011db47) or AME tenant (33e01921-4d64-4f8c-a055-5bdaffd5e33d).
    - Enable “Azure Authentication” during data source setup, select “App Registration” from drop down and provide following details -
      - Directory (tenant) ID: Go to AAD APP in portal, overview page and copy the tenant Id. 
      - Application (client) ID: Go to AAD APP in portal, overview page and copy the Application (client) ID.
      - Client Secret: Go to AAD APP in portal, create a new secret and copy it. You will get 1 time opportunity to copy it after creation.
      - AAD Resource Id: https://management.azure.com
    
     ![Add AMGAddDatasourcePP4](~/metrics/images/prometheus/AMGAddDatasourcePP4.png)

> [!Note]
> If you are using Grafana dogfood version, use this step for Authentication
>  Under Auth section, enable the **Forward OAuth Identity** toggle. This enables with AAD auth from Grafana to MDM to work.  
  
![Add datasource6](~/metrics/images/prometheus/AMGAddDatasource6.png)  
  
- Finally, you need to let the data source know your specific MDM account. This is passed via a custom header. In Custom HTTP headers section, click **+ Add header**  
    - For **Header** enter 'mdmAccountName'  
    - For **Value** enter your actual MDM metrics account  

![Add datasource7](~/metrics/images/prometheus/AMGAddDatasource7.png)  
  
* Click **Save and test** to validate the data source. You are ready once you see a 'Data source is working' confirmation.  

--------------------------------------

In this step you set up Grafana to access Prometheus metrics from your MDM metrics account. Go to this [link](https://grafana.com/docs/grafana/v7.5/datasources/prometheus/) for more information on Prometheus for Grafana  

Next, you will look at how to use the several [built-in dashboards](~/metrics/Prometheus/PromMDMTutorial6ReuseExistingDashboard.md) that are available for you out of the box.
