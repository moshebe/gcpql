.PHONY: build test clean install

build:
	go build -o gcpql

test:
	go test ./... -v

clean:
	rm -f gcpql

install: build
	cp gcpql ~/bin/gcpql

lint:
	go vet ./...
	go fmt ./...
