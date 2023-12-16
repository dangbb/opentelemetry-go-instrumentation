# Sử dụng công cụ 

## Tạo file chạy nhị phân cho công cụ và các service mock 

Đối với công cụ, thực hiện:

```shell
go build cli/main.go
```

Đối với các ứng dụng trong microservice test:

```shell
cd playground/microservices_pure;

go build -o bin/audit service/auditservice/cmd/cmd.go;
go build -o bin/customer service/customer/cmd/cmd.go;
go build -o bin/interceptor service/interceptor/cmd/cmd.go;
go build -o bin/warehouse service/warehouse/cmd/cmd.go;
```

Đối với các ứng dụng trong microservice test sử dụng công cụ helios:

```shell
cd playground/microservices_helios_instrumented;

go build -o bin/audit service/auditservice/cmd/cmd.go;
go build -o bin/customer service/customer/cmd/cmd.go;
go build -o bin/interceptor service/interceptor/cmd/cmd.go;
go build -o bin/warehouse service/warehouse/cmd/cmd.go;
```

## Chạy trên môi trường máy cá nhân 

1. Chỉnh vị trí lưu dữ liệu trong biến `VOLUME_FOLDER` trong `docker/infra/.env`. Dựng các service trên Docker, sử dụng docker compose.

```shell
cd playground/microservices_pure;
docker-compose -f docker/infra/docker-compose-infra.yml up -d;
```

2. Chạy các service test 

```shell
LOGGER_LEVEL=debug GRPC_PORT=8090 MYSQL_HOST=audit-mysql MYSQL_PORT=3306 MYSQL_DBNAME=dbname MYSQL_USERNAME=username MYSQL_PASSWORD=password KAFKA_BROKER=localhost:9092 KAFKA_TOPIC=warehouse MIGRATION_FOLDER=playground/microservices_pure/service/auditservice/migration INTERCEPTOR_ADDRESS=localhost:8090 AUDIT_ADDRESS=localhost:8091 WAREHOUSE_ADDRESS=http://localhost:8092 CUSTOMER_ADDRESS=http://localhost:8093; playground/microservices_pure/bin/interceptor server
```

```shell
LOGGER_LEVEL=debug GRPC_PORT=8091 MYSQL_HOST=audit-mysql MYSQL_PORT=3306 MYSQL_DBNAME=dbname MYSQL_USERNAME=username MYSQL_PASSWORD=password KAFKA_BROKER=localhost:9092 KAFKA_TOPIC=warehouse MIGRATION_FOLDER=playground/microservices_pure/service/auditservice/migration INTERCEPTOR_ADDRESS=localhost:8090 AUDIT_ADDRESS=localhost:8091 WAREHOUSE_ADDRESS=http://localhost:8092 CUSTOMER_ADDRESS=http://localhost:8093; playground/microservices_pure/bin/audit server
```

```shell
LOGGER_LEVEL=debug GRPC_PORT=8092 MYSQL_HOST=audit-mysql MYSQL_PORT=3306 MYSQL_DBNAME=dbname MYSQL_USERNAME=username MYSQL_PASSWORD=password KAFKA_BROKER=localhost:9092 KAFKA_TOPIC=warehouse MIGRATION_FOLDER=playground/microservices_pure/service/auditservice/migration INTERCEPTOR_ADDRESS=localhost:8090 AUDIT_ADDRESS=localhost:8091 WAREHOUSE_ADDRESS=http://localhost:8092 CUSTOMER_ADDRESS=http://localhost:8093; playground/microservices_pure/bin/warehouse server
```

```shell
LOGGER_LEVEL=debug GRPC_PORT=8093 MYSQL_HOST=audit-mysql MYSQL_PORT=3306 MYSQL_DBNAME=dbname MYSQL_USERNAME=username MYSQL_PASSWORD=password KAFKA_BROKER=localhost:9092 KAFKA_TOPIC=warehouse MIGRATION_FOLDER=playground/microservices_pure/service/auditservice/migration INTERCEPTOR_ADDRESS=localhost:8090 AUDIT_ADDRESS=localhost:8091 WAREHOUSE_ADDRESS=http://localhost:8092 CUSTOMER_ADDRESS=http://localhost:8093; playground/microservices_pure/bin/customer server
```

Đối với các service helios thay `microservices_pure` bằng `microservices_helios_instrumented`.

3. Chạy công cụ theo dõi. Ví dụ đối với interceptor

`OTEL_GO_AUTO_TARGET_EXE` chứa full path. E.g:;

```shell
export OTEL_GO_AUTO_TARGET_EXE=/home/dangbb/dangnh-opentelemetry-go-instrumentation/playground/microservices_pure/bin/interceptor;
export OTEL_SERVICE_NAME=interceptor;
sudo -E ./main
```

## Chạy toàn bộ trên docker 

```shell
cd playground/microservices_pure;
docker-compose -f playground/microservices_pure/docker/develop/docker-compose.yml up -d;
```