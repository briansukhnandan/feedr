.PHONY: start stop test shell export

start:
	@if docker compose ps --status running --services | grep -qx feedr; then \
		echo "feedr is already running; aborting."; \
		exit 1; \
	fi
	./scripts/quickstart.sh
	@echo "Started feedr!"

stop:
	docker compose rm --stop --force feedr

test:
	docker build --target test .

shell:
	docker compose exec feedr /bin/sh

export:
	@feedr_log_file="/tmp/feedr_logs_$$(date +%Y_%m_%d_%H_%M_%S).log"; \
	docker compose logs --no-color feedr > "$$feedr_log_file"; \
	printf 'Exported feedr logs to %s\n' "$$feedr_log_file"
