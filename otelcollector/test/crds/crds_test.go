package crds

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "github.com/prometheus/prometheus/discovery/install" // Register service discovery implementations.
)

/*
 * Test that the Prometheus UI /scrape_pools API endpoint returns a list that contains at least the default targets.
 */
var _ = DescribeTable("The Prometheus UI API should return the scrape pools",
  func() {
  },
)
