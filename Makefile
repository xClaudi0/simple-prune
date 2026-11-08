NAME    := simpleprune
ALIAS   := sp
EXE     := $(shell go env GOEXE)
OUT     := bin/docker-$(NAME)$(EXE)
PLUGINS := $(HOME)/.docker/cli-plugins
VERSION ?= 0.1.0

.PHONY: build test install clean

build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(OUT) .

test:
	go vet ./...
	go test ./...

# Installs both `docker simpleprune` and the short `docker sp`.
install: build
	mkdir -p "$(PLUGINS)"
	cp "$(OUT)" "$(PLUGINS)/docker-$(NAME)$(EXE)"
	cp "$(OUT)" "$(PLUGINS)/docker-$(ALIAS)$(EXE)"
	@echo "Installed: docker $(NAME) and docker $(ALIAS) in $(PLUGINS)"

clean:
	rm -rf bin
