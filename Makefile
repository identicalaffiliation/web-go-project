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

.PHONY: keys
keys:
	go -C backend/auth run ./cmd/keygen -out ../../secrets/jwt

.PHONY: keys-force
keys-force:
	go -C backend/auth run ./cmd/keygen -out ../../secrets/jwt -force

.PHONY: run
run:
	docker compose up --build -d auth-service
	docker compose up --build -d catalog-service
	docker compose up --build -d profile-service
	docker compose up -d gateway

.PHONY: nginx-reload
nginx-reload:
	docker compose exec gateway nginx -t
	docker compose exec gateway nginx -s reload

.PHONY: clean
clean:
	docker compose down -v

.PHONY: lint
lint:
	cd backend/catalog && golangci-lint run ./...
	cd backend/auth && golangci-lint run ./...
	cd backend/profile && ruff check .
	cd backend/profile && ruff format --check .
	cd backend/profile && python3 -m compileall -q -x 'internal/adapters/grpc/generated' cmd internal

.PHONY: test
test: unit integrations
	@cd backend/profile && \
	if find . -path ./.venv -prune -o \( -name 'test_*.py' -o -name '*_test.py' \) -print | grep -q .; then \
		python3 -m pytest; \
	else \
		echo "profile: no tests found, skipping"; \
	fi

.PHONY: integrations
integrations:
	cd backend/catalog && go test -v -race -tags=integration ./...
	cd backend/auth && go test -v -race -tags=integration ./...

.PHONY: unit
unit:
	cd backend/catalog && go test -v -race -tags=unit ./...
	cd backend/auth && go test -v -race -tags=unit ./...

.PHONY: gen
gen:
	cd backend/catalog && go generate ./...

.PHONY: swag
swag:
	cd backend/catalog && swag init -g cmd/catalog/main.go -o docs/catalog --parseDependency --parseInternal
