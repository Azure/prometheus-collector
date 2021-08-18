BASE_DIR := $(subst /build/linux,,$(PWD))

all: receivervalidator

.PHONY: receivervalidator
receivervalidator:
	rm -rf receivervalidator
	go get
	go build -o receivervalidator .