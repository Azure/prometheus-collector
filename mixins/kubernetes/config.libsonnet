{
  _config+:: {
    SLOs: {
      apiserver: {
        days: 30,  // The number of days we alert on burning too much error budget for.
        target: 0.99,  // The target percentage of availability between 0-1. (0.99 = 99%, 0.999 = 99.9%)

        // Only change these windows when you really understand multi burn rate errors.
        // Even though you can change the days above (which will change availability calculations)
        // these windows will alert on a 30 days sliding window. We're looking into basing these windows on the given days too.
        windows: [
          { severity: 'critical', 'for': '2m', long: '1h', short: '5m', factor: 14.4 },
          { severity: 'critical', 'for': '15m', long: '6h', short: '30m', factor: 6 },
          { severity: 'warning', 'for': '1h', long: '1d', short: '2h', factor: 3 },
          { severity: 'warning', 'for': '3h', long: '3d', short: '6h', factor: 1 },
        ],
      },
    },

    // Selectors are inserted between {} in Prometheus queries.
    cadvisorSelector: 'job="cadvisor"',
    kubeletSelector: 'job="kubelet"',
    kubeStateMetricsSelector: 'job="kube-state-metrics"',
    nodeExporterSelector: 'job="node"',
    kubeSchedulerSelector: 'job="kube-scheduler"',
    kubeControllerManagerSelector: 'job="kube-controller-manager"',
    kubeApiserverSelector: 'job="kube-apiserver"',
    kubeProxySelector: 'job="kube-proxy"',
    podLabel: 'pod',
    hostNetworkInterfaceSelector: 'device!~"veth.+"',
    hostMountpointSelector: 'mountpoint="/"',
    windowsExporterSelector: 'job="windows-exporter"',
    containerfsSelector: 'container!=""',
    clusterSelector: 'cluster="replace_cluster_name_here"',

    // Grafana dashboard IDs are necessary for stable links for dashboards
    grafanaDashboardIDs: {
      'k8s-resources-multicluster.json': '669757cdd4ef4e97b96164c62ac548af',
      'k8s-resources-cluster.json': 'efa86fd1d0c121a26444b636a3f509a8',
      'k8s-resources-namespace.json': '85a562078cdf77779eaa1add43ccec1e',
      'k8s-resources-pod.json': '6581e46e4e5c7ba40a07646395ef7b23',
      //not-used 'k8s-multicluster-rsrc-use.json': 'NJ9AlnsObVgj9uKiJMeAqfzMi1wihOMupcsDhlhR',
      //not-used 'k8s-cluster-rsrc-use.json': 'uXQldxzqUNgIOUX6FyZNvqgP2vgYb78daNu4GiDc',
      //not-used 'k8s-node-rsrc-use.json': 'E577CMUOwmPsxVVqM9lj40czM1ZPjclw7hGa7OT7',
      // ? Confirm if I need to add the new dashboards here
      //not-used 'nodes.json': 'kcb9C2QDe4IYcjiTOmYyfhsImuzxRcvwWC3YLJPS',
      'persistentvolumesusage.json': '919b92a8e8041bd567af9edab12c840c',
      //not-used 'pods.json': 'AMK9hS0rSbSz7cKjPHcOtk6CGHFjhSHwhbQ3sedK',
      //not-used 'statefulset.json': 'dPiBt0FRG5BNYo0XJ4L0Meoc7DWs9eL40c1CRc1g',
      'k8s-resources-windows-cluster.json': '6438557df4391b100730f2494bccaef3',
      'k8s-resources-windows-namespace.json': '98e54027a2724ab1d4c45666c1fa674e',
      'k8s-resources-windows-pod.json': '56497a7ea5610e936dc6ed374a7ce2e1',
      'k8s-windows-cluster-rsrc-use.json': 'VESDBJS7k',
      'k8s-windows-node-rsrc-use.json': 'YCBDf1I7k',
      'k8s-resources-workloads-namespace.json': 'a87fb0d919ec0ea5f6543124e16c42a5',
      'k8s-resources-workload.json': 'a164a7f0339f99e89cea5cb47e9be617',
      'apiserver.json': '09ec8aa1e996d6ffcd6817bbaff4db1b',
      'controller-manager.json': '3aa700ed75ce4c64ba52ef5ca23f2655',
      'scheduler.json': '0252eb9a5da7445a8787400871546188',
      'proxy.json': '632e265de029684c40b21cb76bca4f94',
      'kubelet.json': '3138fa155d5915769fbded898ac09ff9',
      //newly added
      'workload-total.json': '728bf77cc1166d2f3133bf25846876cc',
      'pod-total.json': '7a18067ce943a40ae25454675c19ff5c',
      'namespace-by-workload.json': 'bbb2a765a623ae38130206c7d94a160f',
      'namespace-by-pod.json': '8b7a8b326d7a6f1f04244066368c67af',
      'k8s-resources-node.json': '200ac8fdbfbb74b39aff88118e4d1c2c',
      'cluster-total.json': 'ff635a025bcfea7bc3dd4f508990a3e9',

    },

    // Support for Grafana 7.2+ `$__rate_interval` instead of `$__interval`
    grafana72: true,
    grafanaIntervalVar: if self.grafana72 then '$__rate_interval' else '$__interval',

    // Config for the Grafana dashboards in the Kubernetes Mixin
    grafanaK8s: {
      dashboardNamePrefix: 'Kubernetes / ',
      dashboardTags: ['kubernetes-mixin'],

      // For links between grafana dashboards, you need to tell us if your grafana
      // servers under some non-root path.
      linkPrefix: '',

      // The default refresh time for all dashboards, default to 10s
      refresh: '1m',
      minimumTimeInterval: '1m',

      // Timezone for Grafana dashboards:: UTC, browser, ...
      grafanaTimezone: 'UTC',
    },

    // Opt-in to multiCluster dashboards by overriding this and the clusterLabel.
    showMultiCluster: true,
    clusterLabel: 'cluster',

    namespaceLabel: 'namespace',

    // Default datasource name
    datasourceName: 'default',

    // Datasource instance filter regex
    datasourceFilterRegex: '',

    // This list of filesystem is referenced in various expressions.
    fstypes: ['ext[234]', 'btrfs', 'xfs', 'zfs'],
    fstypeSelector: 'fstype=~"%s"' % std.join('|', self.fstypes),

    // This list of disk device names is referenced in various expressions.
    diskDevices: ['mmcblk.p.+', 'nvme.+', 'rbd.+', 'sd.+', 'vd.+', 'xvd.+', 'dm-.+', 'dasd.+'],
    diskDeviceSelector: 'device=~"%s"' % std.join('|', self.diskDevices),

    // Certain workloads (e.g. KubeVirt/CDI) will fully utilise the persistent volume they claim
    // the size of the PV will never grow since they consume the entirety of the volume by design.
    // This selector allows an admin to 'pre-mark' the PVC of such a workload (or for any other use case)
    // so that specific storage alerts will not fire.With the default selector, adding a label `exclude-from-alerts: 'true'`
    // to the PVC will have the desired effect.
    pvExcludedSelector: 'label_excluded_from_alerts="true"',

    // Default timeout value for k8s Jobs. The jobs which are active beyond this duration would trigger KubeJobNotCompleted alert.
    kubeJobTimeoutDuration: 12 * 60 * 60,
  },
}
