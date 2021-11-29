{
  _config+:: {
    corednsSelector: 'job="kube-dns"',
    instanceLabel: 'pod',

    grafanaDashboardIDs: {
      'coredns.json': 'vkQ0UHxik',
    },

    pluginNameLabel: 'name',
    kubernetesPlugin: false,
    grafana: {
      dashboardNamePrefix: '',
      dashboardTags: ['coredns-mixin'],

      // The default refresh time for all dashboards, default to 10s
      refresh: '1m',

    },

    // Opt-in for multi-cluster support.
    showMultiCluster: true,
    clusterLabel: 'cluster',
  },
}
