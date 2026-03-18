.PHONY: build test-go dev health-check

build:
	@echo "==> Building"
	@cd hub/collector-engine && go build -o /dev/null ./cmd/... && echo "  OK collector-engine"
	@cd hub/log-pipeline     && go build -o /dev/null ./cmd/... && echo "  OK log-pipeline"
	@cd hub/alert-engine     && go build -o /dev/null ./cmd/... && echo "  OK alert-engine"
	@cd hub/api-server       && go build -o /dev/null ./cmd/... && echo "  OK api-server"

test-go:
	@echo "==> Testing"
	@cd hub/collector-engine && go test ./... -cover -count=1
	@cd hub/log-pipeline     && go test ./... -cover -count=1
	@cd hub/alert-engine     && go test ./... -cover -count=1
	@cd hub/api-server       && go test ./... -cover -count=1

dev:
	@trap 'kill %1 %2 %3 %4 2>/dev/null; exit' INT; \
	(cd hub/collector-engine && PORT=9090 go run ./cmd/...) & \
	(cd hub/log-pipeline     && PORT=9091 go run ./cmd/...) & \
	(cd hub/alert-engine     && PORT=9092 go run ./cmd/...) & \
	(cd hub/api-server       && PORT=8080 go run ./cmd/...) & \
	wait

health-check:
	@curl -sf http://localhost:9090/health && echo " OK collector-engine"
	@curl -sf http://localhost:9091/health && echo " OK log-pipeline"
	@curl -sf http://localhost:9092/health && echo " OK alert-engine"
	@curl -sf http://localhost:8080/health && echo " OK api-server"
