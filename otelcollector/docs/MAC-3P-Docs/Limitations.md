# Limitations


## MAC (Monitoring Account) limitations: 
The default limit on the number of timeseries is 50000.  
The default limit on the number of events is 250000  
  
<br/>

## Prometheus Query Service limitations:  
### **Supported APIs**
You can find full specification of [OSS prom APIs](https://prometheus.io/docs/prometheus/latest/querying/api/) .  We support following:

[Instant queries](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries): /api/v1/query

[Range queries](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries): /api/v1/query_range

[Series](https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers): /api/v1/series

[Labels](https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names): /api/v1/labels

[Label values](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values): /api/v1/label/\_\_name\_\_\/values. It’s the only supported version of this API which effectively means GET all metric names. Any other /api/v1/label/{name}/values **are not supported**.


### **API limitations (differing from prom specification)**
**Case sensitivity**

Azure Prometheus solution is case insensitive whereas PromQL specifies case sensitive matchers. To prevent unexpected results, all case sensitive matchers are treated as case insensitive matchers. Also, all the names/values of metrics or labels that are not present in query itself are returned in lower casing. Any string present in query itself is retuned in the same casing though.
        
    Note: Our storage may store data in different casing for same metric/label in different partitions. If you are using out of the box default dashboards, all the queries there are tested by us to work and they do not face any issues with case insensitive behavior. If you are writing your own queries, please ensure to have all strings in your promQL expression in lower casing

**Scoped to metric**

Any time series fetch queries (**/series** or **/query** or **/query_range)** must contain name label matcher i.e., each query must be scoped to a metric. And there should be exactly one name label matcher in a query, not more than one.

**Supported time range**

**/query_range** API supports a time range of 30 days (end time minus start time).

**/series** API fetches data only for 12 hours time range. If endTime is not provided, endTime = time.now().

**range selectors** (time range baked in query itself) supports 15d. We are evaluating if we can increase this to 30d.

    Note: These supported time ranges are subject to change as we are still experimenting.

**Ignore time range**

Start time and end time provided with **/labels** and **/label**/name/values are ignored, and all retained data in MAC is queried.

**Experimental features**

None of the experimental features are supported such as [exemplars](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-exemplars), [@ Modifier](https://prometheus.io/docs/prometheus/latest/feature_flags/#modifier-in-promql[) or [negative offsets](https://prometheus.io/docs/prometheus/latest/feature_flags/#negative-offset-in-promql).