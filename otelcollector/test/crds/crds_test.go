package crds

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Calling the API server using the Prometheus Operator client", func() {
  It("should get the pod monitor custom resources", func() {
		uuid = "1234"
		fmt.Println("uuid: %s", uuid)
  })

  It("should get the service monitor custom resources", func() {
		uuid = "1234"
		fmt.Println("uuid: %s", uuid)
  })
})
