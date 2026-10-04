.PHONY: start test shell

start:
	./scripts/quickstart.sh
	@echo "Started feedr!"

test:
	docker build --target test .

shell:
	docker compose exec feedr /bin/sh
