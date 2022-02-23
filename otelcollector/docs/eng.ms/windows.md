# Collecting Kubernetes metrics from Windows pods/nodes

#### Collecting Windows Prometheus metrics
    By default the Prometheus collector only collects Kubelet Windows metrics and does not collect any other Windows metrics for Windows pods & Windows nodes in the cluster. To enable Windows metrics collection from default targets, each Windows target need to be enabled by default through the following chart parameters

        - scrapeTargets.windowsExporter
        - scrapeTargets.windowsKubeProxy
  
#### Running natively on Windows nodes when mode.advanced=true
    In addition to running a Replica on Linux node, and Daemonset on Linux nodes (when mode.advanced is enabled), Prometheus collector can also deploy a DaemonSet on Windows nodes, when the following chart parameteres are set to true
        - mode.advanced
        - windowsDaemonset
    It is recommended to run in advanced mode and enable windowsDaemonset on clusters that have >=25 Windows nodes, as this will distribute the default Windows node targets (Windows exporter, Windows kube-proxy, Windows Kubelet)


#### Default metrics for Windows
    Prometheus collector could be configured to collect metrics from following windows targets in Kubernetes clusters
        - Windows exporter
        - Windows kube-proxy
        - Windows Kubelet
    By default only Windows Kubelet is enabled, and you will need to enable Windows exporter and kube-proxy to collect metrics from them. 
    
    Note:- If you have custom metrics exposed by Windows pods, you can always author them to be collected by providing custom scrape job configurations, irrespective of the above Windows specific settings. Instructions are [here]((~/metrics/Prometheus/advanced-mode.md)) about how to do so.
