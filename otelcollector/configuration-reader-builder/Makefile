.PHONY: configurationreader
configurationreader:
	@echo "========================= Building configurationreader ========================="
	@echo "========================= cleanup existing configurationreader ========================="
	rm -rf configurationreader
	@echo "========================= go get  ========================="
	go get
	@echo "========================= go build  ========================="
	go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o configurationreader .