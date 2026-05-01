.PHONY: env-up \
        dev-up dev-up-build dev-down dev-down-soft dev-rebuild dev-restart dev-init logs \
        build push

# ─── Environment ──────────────────────────────────────────────────────────────
env-up:
	cp .env.example .env

# ─── Development ──────────────────────────────────────────────────────────────
dev-up:
	docker compose -f infra/docker-compose.dev.yaml up -d

dev-up-build:
	docker compose -f infra/docker-compose.dev.yaml up -d --build

dev-down:
	docker compose -f infra/docker-compose.dev.yaml down -v

dev-down-soft:
	docker compose -f infra/docker-compose.dev.yaml down

dev-rebuild: dev-down dev-up-build

dev-restart: dev-down dev-up

logs:
	docker compose -f infra/docker-compose.dev.yaml logs -f

dev-init:
	docker compose -f infra/docker-compose.dev.yaml up -d minio minio-init
	$(MAKE) dev-restart

# ─── Production build ─────────────────────────────────────────────────────────
REGISTRY  ?= ghcr.io
OWNER     ?= $(shell git config user.name | tr '[:upper:]' '[:lower:]' | tr ' ' '-')
BRANCH    := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo local)
SHA       := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
SERVICES  := auth core ingestor adapter puller

build:
	@for svc in $(SERVICES); do \
		echo "→ building $$svc"; \
		docker build \
			-t $(REGISTRY)/$(OWNER)/omnigate-$$svc:$(BRANCH) \
			-t $(REGISTRY)/$(OWNER)/omnigate-$$svc:$(BRANCH)-$(SHA) \
			services/$$svc; \
	done

push:
	@for svc in $(SERVICES); do \
		echo "→ pushing $$svc"; \
		docker push $(REGISTRY)/$(OWNER)/omnigate-$$svc:$(BRANCH); \
		docker push $(REGISTRY)/$(OWNER)/omnigate-$$svc:$(BRANCH)-$(SHA); \
	done
