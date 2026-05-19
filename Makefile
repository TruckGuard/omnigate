.PHONY: env-up \
        dev-up dev-up-build dev-down dev-down-soft dev-rebuild dev-restart dev-init logs \
        prod-up prod-pull prod-down prod-down-soft prod-restart prod-logs \
        build push \
        test test-reset

# ─── Environment ──────────────────────────────────────────────────────────────
env-up:
	cp .env.example .env

# ─── Development ──────────────────────────────────────────────────────────────
dev-up:
	docker compose --env-file .env -f infra/docker-compose.dev.yaml up -d

dev-up-build:
	docker compose --env-file .env -f infra/docker-compose.dev.yaml up -d --build

dev-down:
	docker compose --env-file .env -f infra/docker-compose.dev.yaml down -v

dev-down-soft:
	docker compose --env-file .env -f infra/docker-compose.dev.yaml down

dev-rebuild: dev-down dev-up-build

dev-restart: dev-down dev-up

logs:
	docker compose --env-file .env -f infra/docker-compose.dev.yaml logs -f

dev-init:
	docker compose --env-file .env -f infra/docker-compose.dev.yaml up -d minio minio-init
	$(MAKE) dev-restart

test:
	@python test-scripts/test.py

test-reset:
	@python test-scripts/test.py --reset

# ─── Production deploy ────────────────────────────────────────────────────────
prod-pull:
	docker compose --env-file .env -f infra/docker-compose.prod.yaml pull

prod-up:
	docker compose --env-file .env -f infra/docker-compose.prod.yaml up -d --build

prod-down:
	docker compose --env-file .env -f infra/docker-compose.prod.yaml down -v

prod-down-soft:
	docker compose --env-file .env -f infra/docker-compose.prod.yaml down

prod-restart: prod-down-soft prod-up

prod-logs:
	docker compose --env-file .env -f infra/docker-compose.prod.yaml logs -f

# ─── Production build ─────────────────────────────────────────────────────────
REGISTRY  ?= ghcr.io
OWNER     ?= $(shell git config user.name | tr '[:upper:]' '[:lower:]' | tr ' ' '-')
BRANCH    := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo local)
SHA       := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
SERVICES  := auth core ingestor adapter puller frontend

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
