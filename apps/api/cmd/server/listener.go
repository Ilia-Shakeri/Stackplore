package main

import (
	"errors"
	"net"
	"sync"
)

var errInvalidConnectionLimit = errors.New("invalid connection limit")

const maximumAcceptedConnections = 16_384

type limitedListener struct {
	net.Listener
	slots     chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
	closeErr  error
}

func newLimitedListener(listener net.Listener, maximum int) (net.Listener, error) {
	if listener == nil || maximum < 1 || maximum > maximumAcceptedConnections {
		if listener != nil {
			_ = listener.Close()
		}
		return nil, errInvalidConnectionLimit
	}
	return &limitedListener{
		Listener: listener,
		slots:    make(chan struct{}, maximum),
		closed:   make(chan struct{}),
	}, nil
}

func (listener *limitedListener) Accept() (net.Conn, error) {
	select {
	case listener.slots <- struct{}{}:
	case <-listener.closed:
		return nil, net.ErrClosed
	}

	connection, err := listener.Listener.Accept()
	if err != nil {
		<-listener.slots
		return nil, err
	}
	return &limitedConnection{
		Conn: connection,
		release: func() {
			<-listener.slots
		},
	}, nil
}

func (listener *limitedListener) Close() error {
	listener.closeOnce.Do(func() {
		close(listener.closed)
		listener.closeErr = listener.Listener.Close()
	})
	return listener.closeErr
}

type limitedConnection struct {
	net.Conn
	release   func()
	closeOnce sync.Once
	closeErr  error
}

func (connection *limitedConnection) Close() error {
	connection.closeOnce.Do(func() {
		connection.closeErr = connection.Conn.Close()
		connection.release()
	})
	return connection.closeErr
}
