package config

import (
	"fmt"
)

type KafkaConfig struct {
	Broker string `name:"broker" help:"Kafka broker address" env:"KAFKA_BROKER" default:"localhost:9093"`
	Topic  string `name:"topic" help:"Kafka topic" env:"KAFKA_TOPIC" default:"warehouse"`
}

type MySqlConfig struct {
	Host     string `name:"mysql-host" help:"mysql host" env:"MYSQL_HOST" default:"localhost"`
	Port     string `name:"mysql-port" help:"mysql port" env:"MYSQL_PORT" default:"3320"`
	DBName   string `name:"mysql-database-name" help:"mysql DB name" env:"MYSQL_DBNAME" default:"dbname"`
	Username string `name:"mysql-username" help:"mysql username" env:"MYSQL_USERNAME" default:"username"`
	Password string `name:"mysql-password" help:"mysql password" env:"MYSQL_PASSWORD" default:"password"`
}

func (m *MySqlConfig) GetDsn() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True",
		m.Username,
		m.Password,
		m.Host,
		m.Port,
		m.DBName)
}

type Config struct {
	Server struct{} `cmd:"" help:"Start a server"`

	Migrate struct {
		Command string `arg:"" name:"command" enum:"up,down,force,create"`
		Option  string `arg:"" name:"option" optional:""`
	} `cmd:"" help:"Migrate database"`

	MigrationFolder string `name:"migration-folder" help:"Address of migration folder" env:"MIGRATION_FOLDER"`

	LoggerLevel string `name:"logger-level" help:"Logger Level" env:"LOGGER_LEVEL"`

	HttpPort uint64 `name:"http-port" help:"Http Port" env:"HTTP_PORT" default:"8092"`
	GrpcPort uint64 `name:"grpc-port" help:"Grpc Port" env:"GRPC_PORT" default:"8092"`

	MySqlConfig MySqlConfig `kong:"embed,help:'MySQL Config'"`
	KafkaConfig KafkaConfig `kong:"embed,help:'Kafka Config'"`

	InterceptorAddress string `name:"interceptor-address" help:"Address of interceptor" env:"INTERCEPTOR_ADDRESS" default:"localhost:8090"`
	AuditAddress       string `name:"audit-address" help:"Address of audit" env:"AUDIT_ADDRESS" default:"localhost:8091"`
	WarehouseAddress   string `name:"warehouse-address" help:"Address of warehouse" env:"WAREHOUSE_ADDRESS" default:"localhost:8092"`
	CustomerAddress    string `name:"customer-address" help:"Address of customer" env:"CUSTOMER_ADDRESS" default:"localhost:8093"`

	JeagerEndpoint     string `name:"jeager-endpoint" help:"Jeager endpoint" env:"JEAGER_ENDPOINT"`
	PrometheusEndpoint string `name:"prometheus-endpoint" help:"Prometheus endpoint" env:"PROMETHEUS_ENDPOINT"`
}
