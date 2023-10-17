package config

import (
	"fmt"
)

type KafkaConfig struct {
	Broker string `name:"broker" help:"Kafka broker address" env:"KAFKA_BROKER"`
	Topic  string `name:"topic" help:"Kafka topic" env:"KAFKA_TOPIC"`
}

type MySqlConfig struct {
	Host     string `name:"mysql-host" help:"mysql host" env:"MYSQL_HOST"`
	Port     string `name:"mysql-port" help:"mysql port" env:"MYSQL_PORT"`
	DBName   string `name:"mysql-database-name" help:"mysql DB name" env:"MYSQL_DBNAME"`
	Username string `name:"mysql-username" help:"mysql username" env:"MYSQL_USERNAME"`
	Password string `name:"mysql-password" help:"mysql password" env:"MYSQL_PASSWORD"`
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

	HttpPort uint64 `name:"http-port" help:"Http Port" env:"HTTP_PORT"`
	GrpcPort uint64 `name:"grpc-port" help:"Grpc Port" env:"GRPC_PORT"`

	MySqlConfig MySqlConfig `kong:"embed,help:'MySQL Config'"`
	KafkaConfig KafkaConfig `kong:"embed,help:'Kafka Config'"`

	InterceptorAddress string `name:"interceptor-address" help:"Address of interceptor" env:"INTERCEPTOR_ADDRESS"`
	AuditAddress       string `name:"audit-address" help:"Address of audit" env:"AUDIT_ADDRESS"`
	WarehouseAddress   string `name:"warehouse-address" help:"Address of warehouse" env:"WAREHOUSE_ADDRESS"`
	CustomerAddress    string `name:"customer-address" help:"Address of customer" env:"CUSTOMER_ADDRESS"`

	JeagerEndpoint     string `name:"jeager-endpoint" help:"Jeager endpoint" env:"JEAGER_ENDPOINT"`
	PrometheusEndpoint string `name:"prometheus-endpoint" help:"Prometheus endpoint" env:"PROMETHEUS_ENDPOINT"`
}
