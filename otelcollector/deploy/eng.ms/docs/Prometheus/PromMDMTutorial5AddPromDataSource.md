
> [!Note]
> Prometheus metrics in MDM is still in active development. It is only available for a very small set of customers to provide very early feedback - limited private preview. Geneva will open this up for broader preview, after we've had a chance to address feedback received in the current limited preview. If your team has not already been contacted for the limited preview, then you are not yet eligible for this preview. You can also join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly.

# Prometheus data source

For Grafana to access your Prometheus metrics it needs to know where the data is and be able to access it with the right authentication. This is accomplished by a data source. Prometheus data source is supported natively in Grafana. We need to have this be configured to pull metrics from your MDM account instead of a Prometheus tsdb.  
  
To do this follow the instructions below.  
    
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

- Under Auth section, enable the **Forward OAuth Identity** toggle. This enables with AAD auth from Grafana to MDM to work.  
  
![Add datasource6](~/metrics/images/prometheus/AMGAddDatasource6.png)  
  
- Finally, you need to let the data source know your specific MDM account. This is passed via a custom header. In Custom HTTP headers section, click **+ Add header**  
    - For **Header** enter 'mdmAccountName'  
    - For **Value** enter your actual MDM metrics account  

![Add datasource7](~/metrics/images/prometheus/AMGAddDatasource7.png)  
  
* Click **Save and test** to validate the data source. You are ready once you see a 'Data source is working' confirmation.  

--------------------------------------

In this step you set up Grafana to access Prometheus metrics from your MDM metrics account.  

Next, you will look at how to use the several [built-in dashboards](~/metrics/prometheus/PromMDMTutorial6ReuseExistingDashboard.md) that are available for you out of the box.
