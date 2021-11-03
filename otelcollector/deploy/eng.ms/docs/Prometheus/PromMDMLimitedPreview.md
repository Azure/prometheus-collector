> [!Note]
> Prometheus metrics in MDM is still in active development. It is only available for a very small set of customers to provide very early feedback - limited private preview. Geneva will open this up for broader preview, after we've had a chance to address feedback received in the current limited preview. If your team has not already been contacted for the limited preview, then you are not yet eligible for this preview. You can also join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly.

# Prometheus metrics in MDM - Limited Private Preview

[Prometheus](https://prometheus.io/docs/introduction/overview/) is an important metrics solution, especially for applications on Kubernetes. Both Geneva and Azure Monitor will provide native integration for Prometheus interfaces (ingesting the 4 Prom metrics types, and querying via PromQL).

The first step towards this is a Limited Private Preview that we are opening up to small set of customers. We will learn from this and then open up more broadly to all Geneva customers in the Ni semester time frame. Subsequently this will also be added as a capability to Azure Monitor.

Here is a timeline, capabilities and expectations of the Limited private preview

## Limited Preview capabilities

* Dedicated MDM stamp available for all preview evaluation. Teams will create MDM accounts in this stamp.
* Ability to ingest all 4 Prometheus metric types, via our K8sAgent (HELM + manual deployment)
* You can provide customizable config (service discovery supported), with scrape intervals up to 1 sec. Scraped metrics will be stored in the created MDM accounts.
* View the ingested metrics in Grafana via a Prometheus data source and run queries using PromQL queries. Grafana will be available as both a managed offering (Azure Grafana Service - preview) or BYO (stand alone Grafana that you manage).

## Timeline (all dates are 2021)

* Starting 5/24 : Teams will be on-boarded to Limited preview (5 teams across MS)
* 5/24 – 7/15 : Get feedback, address any critical issues and prepare for production scale
* 7/15 - 12/31 (Ni) : Starting July we will iteratively open up to a wider set of 1P customers, leading to a full internal preview, and subsequently have customers move production workloads to the solution.

And finally as we work through details of Ni planning, we will have more details to share on when these will light up:

* Ability to alert on Prom metrics via Geneva alerting
* Ingest directly from Prom server via Remote write
* Support for HOBO scenarios (AAD auth)
* Prometheus operator support via CRD
* Scraping local Windows node level metrics
* Recording rules
* Limited preview for Azure Monitor Container insights ingesting native Prometheus metrics (built-in + custom metrics, billing model outlined)

## Further Reading

[Prometheus metrics in MDM - Tutorial](~/metrics/Prometheus/PromMDMTutorial0.md)  
[Prometheus metrics in MDM - FAQ](~/metrics/Prometheus/PromMDMFAQ.md)  
[Prometheus metrics in MDM - Getting help](https://teams.microsoft.com/l/channel/19%3a0ee871c52d1744b0883e2d07f2066df0%40thread.skype/Prometheus%2520metrics%2520in%2520MDM%2520(Limited%2520Preview)?groupId=5658f840-c680-4882-93be-7cc69578f94e&tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47)  
