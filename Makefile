.PHONY: build test clean install

build:
	go build -o gcp-metrics

test:
	go test ./... -v

clean:
	rm -f gcp-metrics

install: build
	cp gcp-metrics ~/bin/gcp-metrics

lint:
	go vet ./...
	go fmt ./...
