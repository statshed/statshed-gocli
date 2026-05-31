# StatShed CLI (Go) — build, test, install, and packaging helpers.
#
# AIDEV-NOTE: Indentation in recipes MUST be tabs. VERSION is injected into the
# binary via -ldflags; keep it in sync with debian/changelog and the .spec.

VERSION ?= 1.0.2
BINARY  := statshed
PKG     := ./cmd/statshed

PREFIX  ?= /usr/local
DESTDIR ?=
BINDIR   = $(DESTDIR)$(PREFIX)/bin
MANDIR   = $(DESTDIR)$(PREFIX)/share/man/man1
BASHDIR  = $(DESTDIR)$(PREFIX)/share/bash-completion/completions
ZSHDIR   = $(DESTDIR)$(PREFIX)/share/zsh/site-functions
FISHDIR  = $(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d

# Reproducible, statically-linkable build with vendored modules.
GOFLAGS  = -mod=vendor -trimpath
LDFLAGS  = -s -w -X main.version=$(VERSION)

.PHONY: all build test vet fmt man install uninstall clean completions deb rpm dist

all: build

build:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

man:
	@man -l docs/statshed.1 | cat

# Generate shell completions from the built binary into ./completions.
completions: build
	@mkdir -p completions
	./$(BINARY) completion bash > completions/$(BINARY).bash
	./$(BINARY) completion zsh  > completions/_$(BINARY)
	./$(BINARY) completion fish > completions/$(BINARY).fish

install: build completions
	install -d $(BINDIR) $(MANDIR) $(BASHDIR) $(ZSHDIR) $(FISHDIR)
	install -m 0755 $(BINARY) $(BINDIR)/$(BINARY)
	install -m 0644 docs/statshed.1 $(MANDIR)/statshed.1
	install -m 0644 completions/$(BINARY).bash $(BASHDIR)/$(BINARY)
	install -m 0644 completions/_$(BINARY) $(ZSHDIR)/_$(BINARY)
	install -m 0644 completions/$(BINARY).fish $(FISHDIR)/$(BINARY).fish

uninstall:
	rm -f $(BINDIR)/$(BINARY) $(MANDIR)/statshed.1 \
	      $(BASHDIR)/$(BINARY) $(ZSHDIR)/_$(BINARY) $(FISHDIR)/$(BINARY).fish

clean:
	rm -f $(BINARY)
	rm -rf completions dist

# Build a Debian package using the debian/ metadata.
deb:
	packaging/build-deb.sh $(VERSION)

# Build an RPM using packaging/rpm/statshed-cli.spec.
rpm:
	packaging/build-rpm.sh $(VERSION)

# Produce a source tarball (vendored, offline-buildable).
dist:
	@mkdir -p dist
	git archive --format=tar.gz --prefix=statshed-cli-$(VERSION)/ \
	    -o dist/statshed-cli-$(VERSION).tar.gz HEAD
