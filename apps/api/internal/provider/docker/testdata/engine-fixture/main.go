package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

var memory [4 << 20]byte
var counter atomic.Uint64

func main() {
	if len(os.Args) == 2 && os.Args[1] == "health" {
		client := &http.Client{Timeout: time.Second}
		response, err := client.Get("http://127.0.0.1:8080/health")
		if err != nil {
			os.Exit(1)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	for index := range memory {
		memory[index] = byte(index % 251)
	}
	go func() {
		for {
			for range 100_000 {
				counter.Add(1)
			}
			runtime.Gosched()
		}
	}()
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			fmt.Println("fixture stdout ready")
			fmt.Fprintln(os.Stderr, "fixture stderr ready")
		}
	}()
	server := &http.Server{Addr: ":8080", ReadHeaderTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = io.WriteString(writer, "fixture ready\n")
		})}
	if server.ListenAndServe() != nil {
		os.Exit(1)
	}
}
