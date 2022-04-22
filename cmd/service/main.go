package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cangoektas/go-open-telemetry/internal/helper"
	"github.com/cangoektas/go-open-telemetry/pkg/registry"
)

var reg *registry.Registry

func init() {
	rand.Seed(time.Now().UnixNano())
}

var serviceDiscoveryAddr = "http://localhost:8090"

func main() {
	min := 1025
	max := 65535
	randPort := rand.Intn(max-min) + min
	name := os.Getenv("NAME")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/registry", setRegistry)
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

	registerService(name, fmt.Sprintf("localhost:%d", randPort))

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
