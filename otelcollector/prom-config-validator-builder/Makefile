.PHONY: promconfigvalidator
promconfigvalidator:
	@echo "========================= Building prom config validator ========================="
	@echo "========================= cleanup existing promconfigvalidator ========================="
	rm -rf promconfigvalidator
	@echo "========================= go get  ========================="
	go get
	@echo "========================= go build  ========================="
	go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o promconfigvalidator .