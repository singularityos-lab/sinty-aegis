BIN := sinty-aegisd
PREFIX ?= /usr

.PHONY: all build release test vet install clean

all: build

build:
	go build -o $(BIN) ./cmd/sinty-aegisd

release:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/sinty-aegisd

test:
	go test ./...

vet:
	go vet ./...

install: release
	install -Dm755 $(BIN) $(DESTDIR)$(PREFIX)/libexec/$(BIN)
	install -Dm644 packaging/dev.sinty.Aegis1.service $(DESTDIR)$(PREFIX)/share/dbus-1/services/dev.sinty.Aegis1.service
	install -Dm644 packaging/sinty-aegisd.desktop $(DESTDIR)/etc/xdg/autostart/sinty-aegisd.desktop

clean:
	rm -f $(BIN)
