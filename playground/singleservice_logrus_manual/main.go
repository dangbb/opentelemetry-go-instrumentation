package main

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"html"
	"net/http"
	"strconv"
	"sync"

	"github.com/alecthomas/kong"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/sdk/trace"
)

type Config struct {
	Port                 uint64 `name:"port" help:"HTTP server port" env:"HTTP_PORT" default:"8094"`
	OtelExporterEndpoint string `name:"otel_exporter_endpoint" help:"otel_exporter_endpoint" env:"OTEL_EXPORTER_ENDPOINT" default:"localhost:4318"`
	OtelExporterInsecure string `name:"otel_exporter_insecure" help:"otel_exporter_insecure" env:"OTEL_EXPORTER_INSECURE" default:"true"`
	OtelServiceName      string `name:"otel_service_name" help:"otel_exporter_service_name" env:"OTEL_SERVICE_NAME" default:"singleservice_logrus"`
}

var providerSingleton *trace.TracerProvider

func logWithSpan(ctx context.Context, level logrus.Level, format string, args ...interface{}) {
	_, span := providerSingleton.
		Tracer("singleservice_logrus_manual").
		Start(ctx, "logrus.Info")
	defer span.End()

	span.SetAttributes([]attribute.KeyValue{
		{
			Key:   "content",
			Value: attribute.StringValue(fmt.Sprintf(format, args)),
		},
		{
			Key:   "level",
			Value: attribute.Int64Value(int64(level)),
		},
	}...)

	switch level {
	case logrus.DebugLevel:
		logrus.Debugf(format, args)
	case logrus.InfoLevel:
		logrus.Infof(format, args)
	case logrus.WarnLevel:
		logrus.Warnf(format, args)
	case logrus.ErrorLevel:
		logrus.Errorf(format, args)
	}
}

func doLog(ctx context.Context, n int) {
	wg := sync.WaitGroup{}
	wg.Add(n)

	for i := 0; i < n; i++ {
		i := i
		ctx := ctx
		go func() {
			defer wg.Done()
			logWithSpan(ctx, logrus.InfoLevel, "Info at goroutine %d", i)
			logWithSpan(ctx, logrus.DebugLevel, "Debug at goroutine %d", i)
			logWithSpan(ctx, logrus.WarnLevel, "Warn at goroutine %d", i)
			logWithSpan(ctx, logrus.ErrorLevel, "Error at goroutine %d", i)
		}()
	}

	wg.Wait()
}

func main() {
	config := Config{}
	kong.Parse(&config)

	// Init singleton for trace provider
	ctx := context.Background()

	options := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(config.OtelExporterEndpoint),
		otlptracehttp.WithURLPath("v1/traces"),
		otlptracehttp.WithInsecure(),
	}

	exporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(options...))
	if err != nil {
		panic(err)
	}

	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("singleservice_logrus_manual"),
		),
	)
	if err != nil {
		panic(err)
	}

	providerParams := []trace.TracerProviderOption{
		trace.WithBatcher(exporter),
		trace.WithResource(r),
	}

	traceProvider := trace.NewTracerProvider(providerParams...)
	otel.SetTracerProvider(traceProvider)
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	otel.SetTextMapPropagator(propagator)

	providerSingleton = traceProvider

	logrus.SetLevel(logrus.DebugLevel)

	http.HandleFunc("/entry", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := providerSingleton.
			Tracer("singleservice_logrus_manual").
			Start(r.Context(), "net.http - /entry")
		defer span.End()

		n := r.URL.Query().Get("n")
		if n == "" {
			http.Error(w, "n loops not found", http.StatusBadRequest)
		}

		nLoops, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("cannot parse %d", nLoops), http.StatusBadRequest)
		}

		doLog(ctx, int(nLoops))

		fmt.Fprintf(w, "Hello, %q\n", html.EscapeString(r.URL.Path))
	})

	logrus.Infof("Start service test for logrus at: 0.0.0.0:%d", config.Port)
	if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", config.Port), nil); err != nil {
		logrus.Fatalf("cant listen to port %d\n", config.Port)
	}
}
