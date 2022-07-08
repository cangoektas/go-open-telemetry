package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	otelSdkTrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func main() {
	exp, err := otlptracegrpc.New(context.Background())
	if err != nil {
		panic(err)
	}

	tp := otelSdkTrace.NewTracerProvider(
		// WithBatcher creates a BatchSpanProcessor that receives spans
		// asynchronously and forwards them in batches to an exporter in a
		// regular interval
		otelSdkTrace.WithBatcher(exp),
		// OpenTelemetry uses a Resource to represent the entity producing
		// telemetry. The configured Resource is referenced by all the
		// Tracers the TracerProvider creates. Note that the configured
		// service name is different to the library names that we use later
		// on.
		otelSdkTrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("sleep-srv"),
		)),
	)

	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	otel.SetTracerProvider(tp)

	signals := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	port := 8081

	if value, exists := os.LookupEnv("PORT"); exists {
		i, err := strconv.Atoi(value)

		if err == nil {
			port = i
		}
	}

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	mux := http.NewServeMux()
	mux.HandleFunc("/sleep/", sleep)

	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}

	go func() {
		fmt.Printf("Starting server at %d\n", port)

		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				panic(err)
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		<-signals
		server.Shutdown(ctx)
		done <- true
	}()

	<-done
}

func parseNum(req *http.Request) (int, error) {
	numStr := strings.TrimPrefix(req.URL.Path, "/sleep/")
	return strconv.Atoi(numStr)
}

func sleep(w http.ResponseWriter, req *http.Request) {
	newCtx, span := otel.Tracer("sleep-lib").Start(req.Context(), "sleep-handler")
	defer span.End()

	num, err := func(ctx context.Context, req *http.Request) (int, error) {
		_, span := otel.Tracer("sleep-lib").Start(ctx, "parseNum")
		defer span.End()

		num, err := parseNum(req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		span.SetAttributes(attribute.String("num", fmt.Sprintf("%d", num)))

		return num, err
	}(newCtx, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	slept := make(chan bool, 2)

	for i := 0; i < 2; i++ {
		func(ctx context.Context, req *http.Request) error {
			_, span := otel.Tracer("sleep-lib").Start(ctx, fmt.Sprintf("sleep-%d", i))
			defer span.End()

			fmt.Printf("Sleeping %d ms...", num/2)
			time.Sleep(time.Duration(num/2) * time.Millisecond)

			slept <- true

			return nil
		}(newCtx, req)
	}

	<-slept
	<-slept
	w.WriteHeader(http.StatusNoContent)
}
