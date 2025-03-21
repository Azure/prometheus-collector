module github.com/prometheus-collector/certgenerator

go 1.23.7

require (
	github.com/Azure/webhook-tls-manager v1.0.9
	k8s.io/legacy-cloud-providers v0.29.0
)

replace github.com/prometheus-collector/certcreator => ../certcreator

require (
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.29 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.23 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/mocks v0.4.2 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	k8s.io/klog/v2 v2.110.1 // indirect
)
