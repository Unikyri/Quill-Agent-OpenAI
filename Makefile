.PHONY: e2e ingestion-perf ingestion-perf-live

INGESTION_PERF_RUNS ?= 1
INGESTION_PERF_PAGES ?= 50

ifeq ($(INGESTION_PERF_PAGES),50)
PERF_PAGES = 50
else ifeq ($(INGESTION_PERF_PAGES),400)
PERF_PAGES = 400
else ifeq ($(INGESTION_PERF_PAGES),all)
PERF_PAGES = 50 400
else
$(error INGESTION_PERF_PAGES must be 50, 400, or all; got "$(INGESTION_PERF_PAGES)")
endif

E2E_COMPOSE = docker compose -p quill-e2e -f docker-compose.yml -f e2e/docker-compose.e2e.yml
PERF_COMPOSE = docker compose -p quill-perf -f docker-compose.yml -f e2e/docker-compose.e2e.yml

# Runs against the assembled Docker stack, not mocks. Qwen is deliberately a
# required input: this is a local pre-merge proof, not a unit-CI target.
e2e:
	@set -e; set -a; if [ -f .env ]; then . ./.env; fi; set +a; \
		test -n "$$QWEN_API_KEY" || (echo "QWEN_API_KEY is required for the live E2E suite"; exit 1); \
		$(E2E_COMPOSE) down -v --remove-orphans; \
		trap '$(E2E_COMPOSE) down -v --remove-orphans' EXIT; \
		$(E2E_COMPOSE) up -d --build; \
		attempt=0; until curl -fsS http://127.0.0.1:18080/api/v1/health >/dev/null; do \
			attempt=$$((attempt + 1)); if [ $$attempt -ge 60 ]; then echo "Timed out waiting for E2E backend health"; exit 1; fi; sleep 2; \
		done; \
		attempt=0; until curl -fsS http://127.0.0.1:13001/ >/dev/null; do \
			attempt=$$((attempt + 1)); if [ $$attempt -ge 60 ]; then echo "Timed out waiting for E2E frontend"; exit 1; fi; sleep 2; \
		done; \
	cd frontend && QWEN_API_KEY="$$QWEN_API_KEY" PLAYWRIGHT_BASE_URL=http://127.0.0.1:13001 TMPDIR=/tmp npx playwright test --config=../e2e/playwright.config.ts

ingestion-perf:
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; cd backend; go run ./cmd/ingestion-perf -pages 50 -runs 3 -fixture ../artifacts/fixtures/ingestion-50-page.md -output ../artifacts/reports/ingestion-50-page.json
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; cd backend; go run ./cmd/ingestion-perf -pages 400 -runs 3 -fixture ../artifacts/fixtures/ingestion-400-page.md -output ../artifacts/reports/ingestion-400-page.json

# Live runs create an isolated PostgreSQL/Qwen benchmark stack. The runner is
# a first-party image with no workspace mount and no .env in its build context.
# Select exactly one fixture with INGESTION_PERF_PAGES=50 or =400. Use =all
# only when intentionally spending quota on both isolated benchmark stacks.
ingestion-perf-live:
	@set -e; set -a; if [ -f .env ]; then . ./.env; fi; set +a; \
		test -n "$$QWEN_API_KEY" || (echo "QWEN_API_KEY is required for live ingestion performance"; exit 1); \
		trap '$(PERF_COMPOSE) down -v --remove-orphans' EXIT; \
		for page in $(PERF_PAGES); do \
			$(PERF_COMPOSE) down -v --remove-orphans; \
			INGESTION_PERF_PAGES="$$page" INGESTION_PERF_RUNS="$(INGESTION_PERF_RUNS)" $(PERF_COMPOSE) --profile perf up --build --abort-on-container-exit --exit-code-from ingestion-perf ingestion-perf; \
			$(PERF_COMPOSE) down -v --remove-orphans; \
		done
