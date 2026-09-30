.PHONY: infra_up
infra_up:
	docker compose up --build -d auth-postgres
	docker compose up --build -d profile-postgres
	docker compose up --build -d catalog-postgres
	docker compose up --build -d shop-zookeeper
	docker compose up --build -d shop-kafka
	docker compose up --build -d kafka-ui

.PHONY: clean
clean:
	docker compose down -v