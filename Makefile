MODULE        := github.com/identicalaffiliation/web-go-project/backend
PROTO_DIR     := backend/proto
CATALOG_PROTO := v1/catalog/catalog.price.v1.proto
FEEDBACKS_PROTO := v1/catalog/catalog.feedbacks.v1.proto
ORDERS_PROTO  := v1/orders/orders.orders.v1.proto
PAYMENTS_PROTO := v1/payments/payments.balance.v1.proto

GO_SERVICES := catalog auth inventory orders payments

INFRA := \
	auth-postgres \
	profile-postgres \
	catalog-master-postgres \
	catalog-replica-postgres \
	orders-master-postgres \
	orders-replica-postgres \
	inventory-master-postgres \
	payments-master-postgres \
	payments-replica-postgres \
	shop-zookeeper \
	shop-kafka \
	kafka-ui \
	shop-minio \
	shop-redis \
	temporal-postgres \
	shop-temporal \
	temporal-ui

MIGRATORS := \
	auth-migrator \
	profile-migrator \
	catalog-migrator \
	inventory-migrator \
	orders-migrator \
	payments-migrator

SERVICES := \
	auth-service \
	catalog-service \
	inventory-service \
	profile-service \
	orders-service \
	payments-service

.DEFAULT_GOAL := help

##@ Общее

.PHONY: help
help: ## список команд
	@awk 'BEGIN {FS = ":.*##"} \
		/^##@/ {printf "\n%s\n", substr($$0, 5)} \
		/^[a-zA-Z0-9_%-]+:.*##/ {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

##@ Docker

.PHONY: up
up: ## поднять инфраструктуру (бд, kafka, minio, redis, temporal)
	docker compose up --build -d $(INFRA)
	docker compose up --build -d frontend

.PHONY: migrations-up
migrations-up: ## накатить миграции всех бд
	docker compose up --build -d $(MIGRATORS)

.PHONY: init-kafka-topics
init-kafka-topics: ## создать топики kafka
	docker compose up --build -d init-kafka

.PHONY: run
run: ## поднять сервисы и gateway
	docker compose up --build -d $(SERVICES)
	docker compose up -d gateway

.PHONY: nginx-reload
nginx-reload: ## проверить и перечитать конфиг nginx
	docker compose exec gateway nginx -t
	docker compose exec gateway nginx -s reload

.PHONY: clean
clean: ## остановить все и удалить volumes
	docker compose down -v --remove-orphans

##@ Ключи

.PHONY: keys
keys: ## создать jwt ключи в secrets/jwt
	go -C backend/auth run ./cmd/keygen -out ../../secrets/jwt

.PHONY: keys-force
keys-force: ## перезаписать jwt ключи (выданные токены перестанут работать)
	go -C backend/auth run ./cmd/keygen -out ../../secrets/jwt -force

##@ Проверки

.PHONY: lint
lint: ## линтеры всех сервисов
	cd backend/catalog && golangci-lint run ./...
	cd backend/auth && golangci-lint run ./...
	cd backend/inventory && golangci-lint run ./...
	cd backend/orders && golangci-lint run ./...
	cd backend/payments && golangci-lint run ./...
	cd backend/profile && ruff check .
	cd backend/profile && ruff format --check .
	cd backend/profile && python3 -m compileall -q -x 'internal/adapters/grpc/generated' cmd internal

K6_SERVICES := auth catalog orders profile
BASE_URL    ?= http://localhost
# RATE и DURATION по умолчанию берутся из скриптов (catalog 500 rps, остальные 50, 1 минута).
# Перебить для всех: make load RATE=100 DURATION=30s
K6_ENV      := -e BASE_URL=$(BASE_URL) $(if $(RATE),-e RATE=$(RATE)) $(if $(DURATION),-e DURATION=$(DURATION))

.PHONY: load ## нагрузочное тестирование на все сервисы по очереди
load:
	@failed=""; \
	for s in $(K6_SERVICES); do \
		echo "==> k6: $$s"; \
		k6 run $(K6_ENV) load/k6/$$s.js || failed="$$failed $$s"; \
	done; \
	if [ -n "$$failed" ]; then echo "k6 failed:$$failed"; exit 1; fi

.PHONY: test
test: unit integrations ## все тесты
	@cd backend/profile && \
	if find . -path ./.venv -prune -o \( -name 'test_*.py' -o -name '*_test.py' \) -print | grep -q .; then \
		python3 -m pytest; \
	else \
		echo "profile: no tests found, skipping"; \
	fi

.PHONY: unit
unit: ## unit тесты go сервисов
	@for s in $(GO_SERVICES); do \
		(cd backend/$$s && go test -v -race -tags=unit ./...) || exit 1; \
	done

.PHONY: integrations
integrations: ## интеграционные тесты go сервисов (нужен docker)
	@for s in $(GO_SERVICES); do \
		(cd backend/$$s && go test -v -race -tags=integration ./...) || exit 1; \
	done

##@ Кодогенерация

.PHONY: gen
gen: proto ## proto + моки
	cd backend/catalog && go generate ./...
	cd backend/inventory && mockery --config configs/mockery.yml
	cd backend/orders && mockery --config configs/mockery.yml
	cd backend/payments && mockery --config configs/mockery.yml

# клиент catalog для orders генерится отдельно: go_package в proto указывает на модуль catalog
.PHONY: proto
proto: ## сгенерировать go код из proto
	protoc -I $(PROTO_DIR) \
		--go_out=backend/catalog --go_opt=module=$(MODULE)/catalog \
		--go-grpc_out=backend/catalog --go-grpc_opt=module=$(MODULE)/catalog \
		$(PROTO_DIR)/$(CATALOG_PROTO)
	protoc -I $(PROTO_DIR) \
		--go_out=backend/orders --go_opt=module=$(MODULE)/orders \
		--go-grpc_out=backend/orders --go-grpc_opt=module=$(MODULE)/orders \
		$(PROTO_DIR)/$(ORDERS_PROTO)
	protoc -I $(PROTO_DIR) \
		--go_out=backend/orders --go_opt=module=$(MODULE)/orders \
		--go_opt=M$(CATALOG_PROTO)=$(MODULE)/orders/gen/pb/price/v1\;pricev1 \
		--go-grpc_out=backend/orders --go-grpc_opt=module=$(MODULE)/orders \
		--go-grpc_opt=M$(CATALOG_PROTO)=$(MODULE)/orders/gen/pb/price/v1\;pricev1 \
		$(PROTO_DIR)/$(CATALOG_PROTO)
	protoc -I $(PROTO_DIR) \
		--go_out=backend/payments --go_opt=module=$(MODULE)/payments \
		--go-grpc_out=backend/payments --go-grpc_opt=module=$(MODULE)/payments \
		$(PROTO_DIR)/$(PAYMENTS_PROTO)
	protoc -I $(PROTO_DIR) \
    	--go_out=backend/catalog --go_opt=module=$(MODULE)/catalog \
    	--go-grpc_out=backend/catalog --go-grpc_opt=module=$(MODULE)/catalog \
    	$(PROTO_DIR)/$(FEEDBACKS_PROTO)

.PHONY: swag
swag: ## swagger всех сервисов
	cd backend/catalog && swag init -g cmd/catalog/main.go -o docs/catalog --parseDependency --parseInternal
	go -C backend/auth tool swag init -g cmd/auth/swagger.go -o docs/auth --parseDependency --parseInternal
	go -C backend/orders tool swag init -g cmd/orders/swagger.go -o docs/orders --parseInternal
