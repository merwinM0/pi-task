PREFIX ?= $(HOME)/.local
GO ?= go

.PHONY: build test check install clean

build:
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o bin/pi-task ./cmd/pi-task

test:
	$(GO) test -race ./...

check: test
	$(GO) vet ./...

install: build
	install -d "$(DESTDIR)$(PREFIX)/bin"
	@set -eu; target="$(DESTDIR)$(PREFIX)/bin"; \
	  temp=$$(mktemp "$$target/.pi-task.XXXXXX"); \
	  trap 'rm -f "$$temp"' EXIT HUP INT TERM; \
	  install -m 0755 bin/pi-task "$$temp"; \
	  mv -f "$$temp" "$$target/pi-task"
	install -d "$(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d"
	install -m 0644 completions/pi-task.fish "$(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/pi-task.fish"
	@printf 'Installed: %s/bin/pi-task\n' '$(PREFIX)'

clean:
	rm -f bin/pi-task coverage.out
