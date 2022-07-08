package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	signals := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	port := 8081

	if value, exists := os.LookupEnv("PORT"); exists {
		i, err := strconv.Atoi(value)

		if err == nil {
			port = i
		}
	}

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
	num, err := func(req *http.Request) (int, error) {
		return parseNum(req)
	}(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fmt.Printf("Sleeping %d ms...", num)
	time.Sleep(time.Duration(num) * time.Millisecond)

	w.WriteHeader(http.StatusNoContent)
}
