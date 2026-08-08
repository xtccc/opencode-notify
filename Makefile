BINARY := opencode-notify
PREFIX := $(HOME)/.local/bin

.PHONY: build test vet install uninstall clean plugin

build:
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/opencode-notify

test:
	go test ./...

vet:
	go vet ./...

install: build
	install -m 0755 $(BINARY) $(PREFIX)/$(BINARY)

uninstall:
	rm -f $(PREFIX)/$(BINARY)

plugin: build
	$(PREFIX)/$(BINARY) install || ./$(BINARY) install

clean:
	rm -f $(BINARY)