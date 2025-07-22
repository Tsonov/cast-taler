package echo

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	mathrand "math/rand"
	"net"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

var (
	serverAddress = pflag.String("echo-server-address", "echo-server", "Address of echo server")
	echoPort      = pflag.Int("echo-port", 8080, "Port of echo server")
	maxDataSizeMB = pflag.Int("max-data-size-mb", 5, "Maximum data transfered per connection in MB")
	minDataSizeMB = pflag.Int("min-data-size-mb", 1, "Minimum data transfered per connection in MB")
)

const (
	MB               = 1024 * 1024 * 1024
	operationTimeout = 20 * time.Second
)

type EchoClient struct {
	log *slog.Logger
}

func NewEchoClient(log *slog.Logger) *EchoClient {
	return &EchoClient{
		log: log,
	}
}

func (e *EchoClient) Run(ctx context.Context) error {

	for {
		e.log.Info("Connecting to server.")
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", *serverAddress, *echoPort))
		if err != nil {
			e.log.Error("Error connecting to server", Err(err))
			return fmt.Errorf("connecting to server: %w", err)
		}

		bufSize := mathrand.Intn(*maxDataSizeMB) + *minDataSizeMB

		errGroup := errgroup.Group{}
		errGroup.Go(func() error { return e.runSendingLoop(conn, bufSize) })
		errGroup.Go(func() error { return e.runReceivingLoop(conn, bufSize) })

		err = errGroup.Wait()
		conn.Close()
		e.log.Info("Connection done")

		if err != nil {
			e.log.Error("Echo client failed", Err(err))
			e.log.Info("Running reconnect")
			continue
		}
	}

}

func (e *EchoClient) runSendingLoop(conn net.Conn, bufSize int) error {
	buff := make([]byte, bufSize*MB)

	e.log.Info("Sending data", slog.Int("buff-size", bufSize*MB))

	rand.Read(buff)

	conn.SetWriteDeadline(time.Now().Add(operationTimeout))

	written, err := writeAll(conn, buff)
	if err != nil {
		e.log.Error("Error sending data", Err(err))
		return fmt.Errorf("sending data: %w", err)
	}

	e.log.Info("Wrote data", slog.Int("bytes", written))
	err = conn.(*net.TCPConn).CloseWrite()
	if err != nil {
		e.log.Error("Error closing write side of connection", Err(err))
		return fmt.Errorf("closing write side of connection: %w", err)
	}

	return nil
}

func (e *EchoClient) runReceivingLoop(conn net.Conn, bufSize int) error {
	buff := make([]byte, bufSize*MB)

	conn.SetReadDeadline(time.Now().Add(operationTimeout))

	data := 0
	for {
		n, err := conn.Read(buff)
		if err != nil && err != io.EOF {
			e.log.Error("Error reading response", Err(err))
			return fmt.Errorf("reading response: %w", err)
		}
		if err == io.EOF {
			e.log.Info("Done receiving data")
			break
		}
		data += n
	}

	e.log.Info("Received data", slog.Int("bytes", data))

	return nil
}

func writeAll(w io.Writer, buf []byte) (int, error) {
	totalWritten := 0
	for totalWritten < len(buf) {
		n, err := w.Write(buf[totalWritten:])
		if err != nil {
			return totalWritten, err
		}
		totalWritten += n
	}
	return totalWritten, nil
}

func Err(err error) slog.Attr {
	return slog.String("err", err.Error())
}
