package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/cangoektas/go-open-telemetry/pkg/registry"
)

var reg = registry.New()

func main() {
	http.HandleFunc("/register", register)
	http.HandleFunc("/unregister", unregister)
	http.HandleFunc("/services", services)

	http.ListenAndServe("localhost:8090", nil)
}

func register(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	reqJson := &registry.RegisterRequest{}
	_, err := readJson(req, reqJson)
	if err != nil {
		panic(err)
	}

	reg.Register(reqJson.Name, reqJson.Addr)

	w.WriteHeader(http.StatusNoContent)
}

func unregister(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	reqJson := &registry.UnregisterRequest{}
	_, err := readJson(req, reqJson)
	if err != nil {
		panic(err)
	}

	reg.Unregister(reqJson.Name)

	w.WriteHeader(http.StatusNoContent)
}

func services(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	respJson, err := json.Marshal(reg.AddrByName)
	if err != nil {
		panic(err)
	}

	w.Write(respJson)
}

func readJson[T any](req *http.Request, res T) (any, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
