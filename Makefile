.PHONY: build

prepare:
	go get github.com/rakyll/statik

build: prepare
	go build

install: prepare
	go install