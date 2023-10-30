package trace

import (
	logrus "github.com/helios/go-sdk/proxy-libs/helioslogrus"
	"github.com/helios/go-sdk/sdk"
	"go.opentelemetry.io/otel/attribute"
	"os"
)

const (
	TraceEndpoint = "TRACE_ENDPOINT"
	TraceInsecure = "TRACE_INSECURE"
)

func InitTrace(serviceName string) {
	endpoint, exists := os.LookupEnv(TraceEndpoint)
	if !exists {
		endpoint = "localhost:4318"
	}

	insecure, exists := os.LookupEnv(TraceInsecure) // N/Y
	if !exists {
		insecure = "Y"
	}

	kvs := []attribute.KeyValue{}
	kvs = append(kvs, sdk.WithCollectorEndpoint(endpoint))
	if insecure == "Y" {
		kvs = append(kvs, sdk.WithCollectorInsecure())
	}

	_, err := sdk.Initialize(serviceName,
		"123",
		kvs...)
	if err != nil {
		logrus.Fatal(err)
	}
}
