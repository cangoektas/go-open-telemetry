// TODO:
// 2. What does the default sampler do?
// 3. What are span limits?
package main

import (
	"context"
	"fmt"
	"log"
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

// Fibonacci returns the n-th fibonacci number. An error is returned if the
// fibonacci number cannot be represented as a uint64.
func Fibonacci(n uint) (uint64, error) {
	if n <= 1 {
		return uint64(n), nil
	}

	if n > 93 {
		return 0, fmt.Errorf("unsupported fibonacci number %d: too large", n)
	}

	var n2, n1 uint64 = 0, 1
	for i := uint(2); i < n; i++ {
		n2, n1 = n1, n1+n2
	}

	return n2 + n1, nil
}

func main() {
	l := log.New(os.Stdout, "", 0)

	// Exporters are packages that allow telemetry data to be emitted somewhere
	// - either to the console, or to a remote system or collector for further
	// analysis and/or enrichment. Here, we create an OTLP exporter. This
	// exporter is configured using a client satisfying the otlptrace.Client
	// interface. This client handles the transformation of data into wire
	// format and the transmission of that data to the collector.
	exp, err := otlptracegrpc.New(context.Background())
	if err != nil {
		l.Fatal(err)
	}

	// A TracerProvider is a centralized point where instrumentation will get a
	// Tracer from to send telemetry data to. Here, we configure a TraceProvider
	// so that the received data is forwarded to exporters.
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
			semconv.ServiceNameKey.String("fib-srv"),
		)),
	)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			l.Fatal(err)
		}
	}()
	// Register the created TracerProvider globally. This pattern is
	// convenient, but not always appropriate. TracerProviders can be
	// explicitly passed to instrumentation or inferred from a context. For this
	// example using a global provider makes sense, but for more complex or
	// distributed codebases, other ways of passing TracerProviders may make
	// more sense.
	otel.SetTracerProvider(tp)

	signals := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	port := 8080
	if value, exists := os.LookupEnv("PORT"); exists {
		i, err := strconv.Atoi(value)

		if err == nil {
			port = i
		}
	}

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	mux := http.NewServeMux()
	mux.HandleFunc("/fib/", fib)

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
	numStr := strings.TrimPrefix(req.URL.Path, "/fib/")
	return strconv.Atoi(numStr)
}

func fib(w http.ResponseWriter, req *http.Request) {
	// Retrieve an appropriately named Tracer from the global
	// TracerProvider. These Tracers are designed to be associated with one
	// instrumentation library. That way, telemetry they produce can be
	// understood to come from that part of a code base.
	// The Start function creates a named Span and returns a new context.
	// Any new spans created based on the new context, will be children of
	// the created span. If no previous span exists in the current context,
	// the created span will be the "root".
	newCtx, span := otel.Tracer("fib-lib").Start(req.Context(), "fib")
	defer span.End()

	defer req.Body.Close()

	num, err := func(ctx context.Context, req *http.Request) (int, error) {
		_, span := otel.Tracer("fib-lib").Start(ctx, "parseNum")
		defer span.End()

		num, err := parseNum(req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		// Adds an attribute to annotate the span. This annotation is something
		// you can add when you think a user of your application will want to
		// see the state or details about the run environment when looking at
		// telemetry.
		span.SetAttributes(attribute.String("num", fmt.Sprintf("%d", num)))

		return num, err
	}(newCtx, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	f, err := func(ctx context.Context) (uint64, error) {
		_, span := otel.Tracer("fib-lib").Start(ctx, "Fibonacci")
		defer span.End()

		f, err := Fibonacci(uint(num))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return f, err
	}(newCtx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write([]byte(strconv.Itoa(int(f)) + "\n"))
}
