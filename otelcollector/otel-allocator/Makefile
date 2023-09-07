.PHONY: targetallocator
targetallocator:
	@echo "========================= Building targetallocator ========================="
	@echo "========================= cleanup existing targetallocator ========================="
	rm -rf targetallocator
	@echo "========================= go get  ========================="
	go get
	@echo "========================= go build  ========================="
	go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o targetallocator .