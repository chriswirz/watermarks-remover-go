BINARY := wmr
ifeq ($(OS),Windows_NT)
BINARY := wmr.exe
endif

.PHONY: build test vet fmt clean install

build:
	go build -o $(BINARY) ./cmd/wmr

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

install:
	go install ./cmd/wmr

clean:
	rm -f $(BINARY)
