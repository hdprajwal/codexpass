BINARY := codex2key

.PHONY: build test vet fmt clean

build:
	go build -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -f $(BINARY)
