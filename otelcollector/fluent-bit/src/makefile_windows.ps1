function clean_up {
    Remove-Item *.so 
    Remove-Item *.h 
    Remove-Item *~
}

# building fluent-bit plugin
Write-Output "========================= Building  out_appinsights plugin go code  ========================="
#$env:BUILDVERSION=$(CONTAINER_BUILDVERSION_MAJOR).$(CONTAINER_BUILDVERSION_MINOR).$(CONTAINER_BUILDVERSION_PATCH)-$(CONTAINER_BUILDVERSION_BUILDNR)
#$env:BUILDDATE=$(CONTAINER_BUILDVERSION_DATE)
#echo $(BUILDVERSION)
#echo $(BUILD_DATE)
Write-Output "========================= cleanup existing .so and .h file  ========================="
clean_up
Write-Output "========================= go get  ========================="
go get
Write-Output "========================= go build  ========================="
go build -buildmode=c-shared -o out_appinsights.so .
