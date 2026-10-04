.PHONY: up
up:
	docker compose up --build -d auth-postgres
	docker compose up --build -d profile-postgres
	docker compose up --build -d catalog-master-postgres
	docker compose up --build -d catalog-replica-postgres
	docker compose up --build -d shop-zookeeper
	docker compose up --build -d shop-kafka
	docker compose up --build -d kafka-ui
	docker compose up --build -d shop-minio
	docker compose up --build -d shop-redis

.PHONY: migration-up
migrations-up:
	docker compose up --build -d auth-migrator
	docker compose up --build -d profile-migrator
	docker compose up --build -d catalog-migrator

.PHONY: init-kafka-topics
init_kafka_topics:
	docker compose up --build -d init-kafka

.PHONY: run
run:
	docker compose up --build -d catalog-service

.PHONY: clean
clean:
	docker compose down -v

.PHONY: lint
lint:
	cd backend/catalog && golangci-lint run ./...

.PHONY: test
test: unit integrations

.PHONY: integrations
integrations:
	cd backend/catalog && go test -v -race -tags=integration ./...

.PHONY: unit
unit:
	cd backend/catalog && go test -v -race -tags=unit ./...

.PHONY: gen
gen:
	cd backend/catalog && go generate ./...