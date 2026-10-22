MODULE      := github.com/identicalaffiliation/web-go-project/backend
PROTO_DIR   := backend/proto
CATALOG_PROTO := v1/catalog/catalog.price.v1.proto

.PHONY: up
up:
	docker compose up --build -d auth-postgres
	docker compose up --build -d profile-postgres
	docker compose up --build -d catalog-master-postgres
	docker compose up --build -d inventory-master-postgres
	docker compose up --build -d catalog-replica-postgres
	docker compose up --build -d shop-zookeeper
	docker compose up --build -d shop-kafka
	docker compose up --build -d kafka-ui
	docker compose up --build -d shop-minio
	docker compose up --build -d shop-redis
	docker compose up --build -d temporal-postgres
	docker compose up --build -d shop-temporal
	docker compose up --build -d temporal-ui

.PHONY: migration-up
migrations-up:
	docker compose up --build -d auth-migrator
	docker compose up --build -d profile-migrator
	docker compose up --build -d catalog-migrator
	docker compose up --build -d inventory-migrator

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
	docker compose up --build -d inventory-service
	docker compose up --build -d profile-service
	docker compose up -d gateway

.PHONY: nginx-reload
nginx-reload:
	docker compose exec gateway nginx -t
	docker compose exec gateway nginx -s reload

.PHONY: clean
clean:
	docker compose down -v --remove-orphans

.PHONY: lint
lint:
	cd backend/catalog && golangci-lint run ./...
	cd backend/auth && golangci-lint run ./...
	cd backend/inventory && golangci-lint run ./...
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
	cd backend/inventory && go test -v -race -tags=integration ./...

.PHONY: unit
unit:
	cd backend/catalog && go test -v -race -tags=unit ./...
	cd backend/auth && go test -v -race -tags=unit ./...
	cd backend/inventory && go test -v -race -tags=unit ./...

.PHONY: proto
proto:
	protoc -I $(PROTO_DIR) \
		--go_out=backend/catalog --go_opt=module=$(MODULE)/catalog \
		--go-grpc_out=backend/catalog --go-grpc_opt=module=$(MODULE)/catalog \
		$(PROTO_DIR)/$(CATALOG_PROTO)

.PHONY: gen
gen: proto
	cd backend/catalog && go generate ./...
	cd backend/inventory && mockery --config configs/mockery.yml

.PHONY: swag
swag:
	cd backend/catalog && swag init -g cmd/catalog/main.go -o docs/catalog --parseDependency --parseInternal

.PHONY: swag-auth
swag-auth:
	go -C backend/auth tool swag init -g cmd/auth/swagger.go -o docs/auth --parseDependency --parseInternal
