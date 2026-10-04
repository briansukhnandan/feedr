.PHONY: start test shell export

start:
	./scripts/quickstart.sh
	@echo "Started feedr!"

test:
	docker build --target test .

shell:
	docker compose exec feedr /bin/sh

export:
	@feedr_log_file="/tmp/feedr_logs_$$(date +%Y_%m_%d_%H_%M_%S).log"; \
	docker compose logs --no-color feedr > "$$feedr_log_file"; \
	printf 'Exported feedr logs to %s\n' "$$feedr_log_file"
