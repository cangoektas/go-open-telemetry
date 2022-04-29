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
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	otelSdkTrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const name = "fib"

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

	exp, err := zipkin.New(
		"http://zipkin:9411/api/v2/spans",
		zipkin.WithLogger(l),
	)
	if err != nil {
		l.Fatal(err)
	}

	tp := otelSdkTrace.NewTracerProvider(
		otelSdkTrace.WithBatcher(exp),
		otelSdkTrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(name),
		)),
	)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			l.Fatal(err)
		}
	}()
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

func parseN(req *http.Request) (int, error) {
	nStr := strings.TrimPrefix(req.URL.Path, "/fib/")
	return strconv.Atoi(nStr)
}

func fib(w http.ResponseWriter, req *http.Request) {
	newCtx, span := otel.Tracer(name).Start(req.Context(), "fib")
	defer span.End()

	defer req.Body.Close()

	n, err := func(ctx context.Context, req *http.Request) (int, error) {
		_, span := otel.Tracer(name).Start(ctx, "parseN")
		defer span.End()

		n, err := parseN(req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		span.SetAttributes(attribute.String("request.n", fmt.Sprintf("%d", n)))

		return n, err
	}(newCtx, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	f, err := func(ctx context.Context) (uint64, error) {
		_, span := otel.Tracer(name).Start(ctx, "Fibonacci")
		defer span.End()

		f, err := Fibonacci(uint(n))
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
