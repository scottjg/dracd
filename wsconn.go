package main

import (
	"github.com/gorilla/websocket"
	"io"
	"net"
	"time"
)

type WsConn struct {
	conn        *websocket.Conn
	reader      io.Reader
	messageType int
}

func CreateWsConn(conn *websocket.Conn) WsConn {
	return WsConn{conn: conn, reader: nil}
}

// Read reads data from the connection.
// Read can be made to time out and return a Error with Timeout() == true
// after a fixed time limit; see SetDeadline and SetReadDeadline.
func (w WsConn) Read(b []byte) (n int, err error) {
	if w.reader == nil {
		w.messageType, w.reader, err = w.conn.NextReader()
		if err != nil {
			return 0, err
		}
	}
	n, err = w.reader.Read(b)
	if err == io.EOF {
		w.reader = nil
	}

	return n, err
}

// Write writes data to the connection.
// Write can be made to time out and return a Error with Timeout() == true
// after a fixed time limit; see SetDeadline and SetWriteDeadline.
func (w WsConn) Write(b []byte) (n int, err error) {
	err = w.conn.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (w WsConn) Close() error {
	return w.conn.Close()
}

// LocalAddr returns the local network address.
func (w WsConn) LocalAddr() net.Addr {
	return w.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (w WsConn) RemoteAddr() net.Addr {
	return w.conn.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail with a timeout (see type Error) instead of
// blocking. The deadline applies to all future I/O, not just
// the immediately following call to Read or Write.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (w WsConn) SetDeadline(t time.Time) error {
	panic("SetDeadline not implemented")
	return nil
}

// SetReadDeadline sets the deadline for future Read calls.
// A zero value for t means Read will not time out.
func (w WsConn) SetReadDeadline(t time.Time) error {
	return w.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (w WsConn) SetWriteDeadline(t time.Time) error {
	return w.conn.SetWriteDeadline(t)
}
