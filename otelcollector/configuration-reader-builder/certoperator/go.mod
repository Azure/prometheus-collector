module github.com/prometheus-collector/certoperator

go 1.23.7

require github.com/prometheus-collector/certgenerator v0.0.0-00010101000000-000000000000

replace github.com/prometheus-collector/certgenerator => ../certgenerator

replace github.com/prometheus-collector/certcreator => ../certcreator

require github.com/prometheus-collector/certcreator v0.0.0-00010101000000-000000000000 // indirect
