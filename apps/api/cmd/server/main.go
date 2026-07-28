package main

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
)

const (
	defaultHTTPAddress = "127.0.0.1:8080"
	readHeaderTimeout  = 5 * time.Second
)

var errInvalidHTTPAddress = errors.New("HTTP address must use a loopback host and a port from 1 to 65535")

func main() {
	address, err := listenerAddress(os.Getenv)
	if err != nil {
		slog.Error("invalid Stackplore API listener configuration", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              address,
		Handler:           httpapi.NewHandler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	slog.Info("starting Stackplore API", "address", address)

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Stackplore API stopped", "error", err)
		os.Exit(1)
	}
}

func listenerAddress(getenv func(string) string) (string, error) {
	address := getenv("STACKPLORE_HTTP_ADDRESS")
	if address == "" {
		address = defaultHTTPAddress
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil || !isLoopbackHost(host) {
		return "", errInvalidHTTPAddress
	}

	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return "", errInvalidHTTPAddress
	}

	return address, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
