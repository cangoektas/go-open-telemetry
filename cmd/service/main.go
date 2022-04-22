package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cangoektas/go-open-telemetry/pkg/registry"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var serviceDiscoveryUrl = "http://localhost:8090"

func main() {
	min := 1025
	max := 65535
	randPort := rand.Intn(max-min) + min
	name := os.Getenv("NAME")
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool, 1)

	go func() {
		<-sigs
		unregisterService(name)
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

	req, err := http.NewRequest(http.MethodPost, serviceDiscoveryUrl+"/register", bytes.NewBuffer(body))
	if err != nil {
		panic(err)
	}

	client := &http.Client{}

	return client.Do(req)
}

func unregisterService(name string) (*http.Response, error) {
	unregisterReq := &registry.UnregisterRequest{
		Name: name,
	}
	body, err := json.Marshal(unregisterReq)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest(http.MethodPost, serviceDiscoveryUrl+"/unregister", bytes.NewBuffer(body))
	if err != nil {
		panic(err)
	}

	client := &http.Client{}

	return client.Do(req)
}
