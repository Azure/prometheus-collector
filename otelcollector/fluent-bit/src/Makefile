fbplugin:
	@echo "========================= Building  out_appinsights plugin go code  ========================="
	#export BUILDVERSION=$(CONTAINER_BUILDVERSION_MAJOR).$(CONTAINER_BUILDVERSION_MINOR).$(CONTAINER_BUILDVERSION_PATCH)-$(CONTAINER_BUILDVERSION_BUILDNR)
	#export BUILDDATE=$(CONTAINER_BUILDVERSION_DATE)
	#@echo $(BUILDVERSION)
	#@echo $(BUILD_DATE)
	@echo "========================= cleanup existing .so and .h file  ========================="
	rm -rf *.so *.h *~
	@echo "========================= go get  ========================="
	go get
	@echo "========================= go build  ========================="
	go build -buildmode=c-shared -ldflags '-Wl,-z,now' -o out_appinsights.so .

clean:
	rm -rf *.so *.h *~
