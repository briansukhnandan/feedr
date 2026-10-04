.PHONY: start test shell

start:
	./scripts/quickstart.sh
	@echo "Started feedr!"

test:
	go test ./...

shell:
	docker compose exec feedr /bin/sh
