package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cangoektas/go-open-telemetry/internal/helper"
	"github.com/cangoektas/go-open-telemetry/pkg/registry"
)

var reg = registry.New()

func main() {
	http.HandleFunc("/register", register)
	http.HandleFunc("/unregister", unregister)
	http.HandleFunc("/registry", getRegistry)

	http.ListenAndServe("localhost:8090", nil)
}

func register(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	reqJson := &registry.RegisterRequest{}
	err := helper.JsonRead(req.Body, reqJson)
	if err != nil {
		panic(err)
	}

	reg.Register(reqJson.Name, reqJson.Addr)
	go updateServices()

	w.WriteHeader(http.StatusNoContent)
}

func unregister(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	reqJson := &registry.UnregisterRequest{}
	err := helper.JsonRead(req.Body, reqJson)
	if err != nil {
		panic(err)
	}

	reg.Unregister(reqJson.Name)
	go updateServices()

	w.WriteHeader(http.StatusNoContent)
}

func getRegistry(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	respJson, err := json.Marshal(reg)
	if err != nil {
		panic(err)
	}

	w.Write(respJson)
}

func updateServices() {
	for name, addr := range reg.AddrByName {
		name := name
		addr := addr
		go updateService(name, addr)
	}
}

func updateService(name string, addr string) (*http.Response, error) {
	body, err := json.Marshal(reg)
	if err != nil {
		panic(err)
	}

	return http.Post(fmt.Sprintf("http://%s/registry", addr), "application/json", bytes.NewBuffer(body))
}
