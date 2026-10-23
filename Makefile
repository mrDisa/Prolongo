.PHONY: setup env up down logs reset check-authors openapi lint-openapi mock

## Run once after cloning: enables the git hooks and creates .env
setup:
	chmod +x .githooks/commit-msg .githooks/pre-push
	git config core.hooksPath .githooks
	git config core.quotepath false
	@[ -f .env ] || cp .env.example .env
	@echo "Готово: хук включён, .env создан."

env:
	@[ -f .env ] || cp .env.example .env

up: env
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=100

## Wipes the database volume: data is reloaded from data/test-data on next start
reset:
	docker compose down -v

## Lists every author and co-author in main — must be only the team
check-authors:
	git log main --format='%an <%ae>%n%(trailers:key=Co-authored-by,valueonly)' | sort -u

## Copies the root openapi.yml into backend-go for go:embed (run after every contract change)
openapi:
	cp openapi.yml backend-go/openapi/openapi.yml

## Lints the API contract (same check as CI)
lint-openapi:
	npx -y @redocly/cli@1.34.22 lint openapi.yml

## Mock API from openapi.yml examples on http://127.0.0.1:4010 (for the frontend)
mock:
	npx -y @stoplight/prism-cli@5 mock openapi.yml -p 4010
