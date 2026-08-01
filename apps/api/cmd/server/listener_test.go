package main

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestLimitedListenerCapsAcceptedConnections(t *testing.T) {
	t.Parallel()
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	listener, err := newLimitedListener(raw, 1)
	if err != nil {
		t.Fatalf("newLimitedListener() error = %v", err)
	}
	defer listener.Close()

	firstAccepted := make(chan net.Conn, 1)
	go func() {
		connection, _ := listener.Accept()
		firstAccepted <- connection
	}()
	firstClient, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("first dial error = %v", err)
	}
	defer firstClient.Close()
	firstServer := <-firstAccepted
	if firstServer == nil {
		t.Fatal("first accepted connection is nil")
	}

	secondAccepted := make(chan net.Conn, 1)
	go func() {
		connection, _ := listener.Accept()
		secondAccepted <- connection
	}()
	secondClient, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		_ = firstServer.Close()
		t.Fatalf("second dial error = %v", err)
	}
	defer secondClient.Close()
	select {
	case connection := <-secondAccepted:
		if connection != nil {
			_ = connection.Close()
		}
		_ = firstServer.Close()
		t.Fatal("second connection was accepted before a slot opened")
	case <-time.After(30 * time.Millisecond):
	}

	if err := firstServer.Close(); err != nil {
		t.Fatalf("first server close error = %v", err)
	}
	select {
	case secondServer := <-secondAccepted:
		if secondServer == nil {
			t.Fatal("second accepted connection is nil")
		}
		_ = secondServer.Close()
	case <-time.After(time.Second):
		t.Fatal("second connection was not accepted after slot release")
	}
}

func TestLimitedListenerCloseWakesCapacityWaiter(t *testing.T) {
	t.Parallel()
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	listener, err := newLimitedListener(raw, 1)
	if err != nil {
		t.Fatalf("newLimitedListener() error = %v", err)
	}

	accepted := make(chan net.Conn, 1)
	go func() {
		connection, _ := listener.Accept()
		accepted <- connection
	}()
	client, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("dial error = %v", err)
	}
	defer client.Close()
	serverConnection := <-accepted
	if serverConnection == nil {
		t.Fatal("accepted connection is nil")
	}
	defer serverConnection.Close()

	waiter := make(chan error, 1)
	go func() {
		_, acceptErr := listener.Accept()
		waiter <- acceptErr
	}()
	if err := listener.Close(); err != nil {
		t.Fatalf("listener close error = %v", err)
	}
	select {
	case acceptErr := <-waiter:
		if !errors.Is(acceptErr, net.ErrClosed) {
			t.Errorf("Accept() error = %v; want net.ErrClosed", acceptErr)
		}
	case <-time.After(time.Second):
		t.Fatal("capacity waiter did not wake on close")
	}
}

func TestLimitedListenerRejectsInvalidOptions(t *testing.T) {
	t.Parallel()
	if _, err := newLimitedListener(nil, 1); !errors.Is(err, errInvalidConnectionLimit) {
		t.Errorf("nil-listener error = %v", err)
	}
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	if _, err := newLimitedListener(raw, 0); !errors.Is(err, errInvalidConnectionLimit) {
		t.Errorf("zero-limit error = %v", err)
	}
	raw, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("second net.Listen() error = %v", err)
	}
	if _, err := newLimitedListener(raw, maximumAcceptedConnections+1); !errors.Is(err, errInvalidConnectionLimit) {
		t.Errorf("large-limit error = %v", err)
	}
}
