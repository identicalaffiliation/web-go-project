.PHONY: infra_up
infra_up:
	docker compose up --build -d auth-postgres
	docker compose up --build -d profile-postgres
	docker compose up --build -d catalog-master-postgres
	docker compose up --build -d catalog-replica-postgres
	docker compose up --build -d shop-zookeeper
	docker compose up --build -d shop-kafka
	docker compose up --build -d kafka-ui

.PHONY: migrate-up
migrate-up:
	docker compose up --build -d auth-migrator
	docker compose up --build -d profile-migrator
	docker compose up --build -d catalog-migrator

.PHONY: init_kafka_topics
init_kafka_topics:
	docker compose up --build -d init-kafka

.PHONY: clean
clean:
	docker compose down -v