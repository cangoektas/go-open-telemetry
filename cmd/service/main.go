package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cangoektas/go-open-telemetry/internal/helper"
	"github.com/cangoektas/go-open-telemetry/pkg/registry"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

var reg *registry.Registry
var randPort int
var name string

func init() {
	rand.Seed(time.Now().UnixNano())
	var (
		min = 1025
		max = 65535
	)
	randPort = rand.Intn(max-min) + min
	name = os.Getenv("NAME")
}

var serviceDiscoveryAddr = "http://localhost:8090"

func main() {
	l := log.New(os.Stdout, "", 0)
	// Write telemetry data to a file.
	f, err := os.Create(fmt.Sprintf("%s-traces.txt", name))
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()

	exp, err := newExporter(f)
	if err != nil {
		l.Fatal(err)
	}

	tc := propagation.TraceContext{}
	// Register the TraceContext propagator globally.
	otel.SetTextMapPropagator(tc)
	otelhttp.WithPropagators(tc)

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(newResource()),
	)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			l.Fatal(err)
		}
	}()

	otel.SetTracerProvider(tp)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/registry", setRegistry)

	helloHandler := http.HandlerFunc(hello)
	wrapperHelloHandler := otelhttp.NewHandler(helloHandler, "hello-instrumented")
	mux.HandleFunc("/hello", func(w http.ResponseWriter, req *http.Request) {
		wrapperHelloHandler.ServeHTTP(w, req)
	})
	server := &http.Server{Addr: fmt.Sprintf("localhost:%d", randPort), Handler: mux}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				panic(err)
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		<-sigs
		unregisterService(name)
		server.Shutdown(ctx)
		done <- true
	}()

	localAddr := fmt.Sprintf("localhost:%d", randPort)

	fmt.Printf("Starting server at %s\n", localAddr)
	registerService(name, localAddr)

	<-done
}

func registerService(name string, addr string) (*http.Response, error) {
	registerReq := &registry.RegisterRequest{
		Name: name,
		Addr: addr,
	}
	body, err := json.Marshal(registerReq)
	if err != nil {
		panic(err)
	}

	return http.Post(serviceDiscoveryAddr+"/register", "application/json", bytes.NewBuffer(body))
}

func unregisterService(name string) (*http.Response, error) {
	unregisterReq := &registry.UnregisterRequest{
		Name: name,
	}
	body, err := json.Marshal(unregisterReq)
	if err != nil {
		panic(err)
	}

	return http.Post(serviceDiscoveryAddr+"/unregister", "application/json", bytes.NewBuffer(body))
}

func setRegistry(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	reg = registry.New()
	err := helper.JsonRead(req.Body, reg)
	if err != nil {
		panic(err)
	}
}

func hello(w http.ResponseWriter, req *http.Request) {
	newCtx, span := otel.Tracer(name).Start(req.Context(), "hello")

	defer req.Body.Close()
	reqDump, err := httputil.DumpRequest(req, false)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(reqDump))

	if len(reg.AddrByName) == 1 || rand.Float32() < 0.1 {
		w.Write([]byte("Hello, world!\n"))
		span.End()
		return
	}

	// Sleep up to 250 milliseconds
	time.Sleep(time.Duration(rand.Intn(251)) * time.Millisecond)

	addrs := make([]string, 0)
	for serviceName, addr := range reg.AddrByName {
		if serviceName != name {
			addrs = append(addrs, addr)
		}
	}

	randIndex := rand.Intn(len(addrs))
	randAddr := addrs[randIndex]

	nextReq, err := http.NewRequest(http.MethodGet, "http://"+randAddr+"/hello", nil)
	if err != nil {
		panic(err)
	}

	nextReq = nextReq.WithContext(newCtx)
	defaultClient := http.DefaultClient

	resp, err := defaultClient.Do(nextReq)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	io.Copy(w, resp.Body)
	span.End()
}

// newExporter returns a console exporter.
func newExporter(w io.Writer) (trace.SpanExporter, error) {
	return stdouttrace.New(
		stdouttrace.WithWriter(w),
		// Use human-readable output.
		stdouttrace.WithPrettyPrint(),
		// Do not print timestamps for the demo.
		stdouttrace.WithoutTimestamps(),
	)
}

// newResource returns a resource describing this application.
func newResource() *resource.Resource {
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(name),
			semconv.ServiceVersionKey.String("v0.1.0"),
			attribute.String("environment", "demo"),
		),
	)
	return r
}
