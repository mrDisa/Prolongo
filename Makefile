.PHONY: setup env up down logs reset check-authors

## Run once after cloning: enables the commit-msg hook and creates .env
setup:
	chmod +x .githooks/commit-msg
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
