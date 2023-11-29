{
  _config+:: {
    corednsSelector: 'job="kube-dns"',
    instanceLabel: 'pod',

    grafanaDashboardIDs: {
      'coredns.json': 'ddcc78cf776f4f5f97660c85e1e96738',
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
