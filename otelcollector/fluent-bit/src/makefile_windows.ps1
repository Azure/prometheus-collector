function clean_up {
    Remove-Item *.so 
    Remove-Item *.h 
    Remove-Item *~
}

Write-Output "========================= cleanup existing .so and .h file  ========================="
clean_up
Write-Output "========================= Building  out_appinsights plugin go code  ========================="
Write-Output "========================= go get  ========================="
go get
Write-Output "========================= go build  ========================="
go build -buildmode=c-shared -o out_appinsights.so .
