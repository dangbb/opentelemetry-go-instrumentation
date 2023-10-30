generate:
	protoc --go_out=pb --go_opt=paths=source_relative --go-grpc_out=pb --go-grpc_opt=paths=source_relative proto/audit.proto

audit-migrate:
	echo \# make migrate-create name="$(name)"
	go run service/auditservice/cmd/main.go migrate create $(name)

.PHONY: generate migrate-create