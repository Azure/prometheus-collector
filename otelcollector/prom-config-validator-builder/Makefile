.PHONY: promconfigvalidator
promconfigvalidator:
	@echo "========================= Building prom config validator ========================="
	@echo "========================= cleanup existing promconfigvalidator ========================="
	rm -rf promconfigvalidator
	@echo "========================= go get  ========================="
	go get
	@echo "========================= go build  ========================="
	go build -ldflags "-s -w" -buildmode=pie -o promconfigvalidator .