package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
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
	mux.HandleFunc("/hello", hello)

	server := &http.Server{Addr: fmt.Sprintf("localhost:%d", port), Handler: mux}

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

func hello(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	w.Write([]byte("Hello, world!\n"))
}
