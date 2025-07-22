package echo

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"reflect"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

const (
	runInfinite = 0
)

var (
	allowReconnect = pflag.Bool("echo-allow-reconnect", false, "Allow to echo client/server to reconnect")
	serverAddress  = pflag.String("echo-server-ip", "echo-server", "IP of echo server")
	echoPort       = pflag.Int("echo-port", 8080, "Port of echo server")
)

const (
	bufSize          = 10 * 1024
	operationTimeout = 20 * time.Second

	iterationsAfterDone = 10
)

type EchoClient struct {
	log *slog.Logger

	reconnected bool
}

func NewEchoClient(log *slog.Logger) *EchoClient {
	return &EchoClient{
		log: log,
	}
}

func (e *EchoClient) Run(ctx context.Context) error {
	dataSend := make([]byte, 0)
	dataReceived := make([]byte, 0)

	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", *serverAddress, *echoPort))
		if err != nil {
			e.log.Error("Error connecting to server", Err(err))
			return fmt.Errorf("connecting to server: %w", err)
		}

		errGroup := errgroup.Group{}
		errGroup.Go(func() error { return e.runSendingLoop(ctx, conn, &dataSend) })
		errGroup.Go(func() error { return e.runReceivingLoop(conn, &dataReceived) })

		err = errGroup.Wait()
		conn.Close()

		if err != nil {
			e.log.Error("Echo client failed", Err(err))

			if !*allowReconnect {
				return fmt.Errorf("echo client failed: %w", err)
			}

			if e.reconnected {
				return fmt.Errorf("echo client already reconnected: %w", err)
			}

			// reset buffers as they are not valid anymore as we might hit situation when we correctly sent
			// data but did not receive it hence the buffers might differs
			dataSend = make([]byte, 0)
			dataReceived = make([]byte, 0)
			e.log.Info("Running reconnect")
			e.reconnected = true
			continue
		}
		break
	}

	if len(dataSend) != 0 && reflect.DeepEqual(dataSend, dataReceived) {
		e.log.Info("Data sent and received match")
	} else {
		e.log.Error("Data sent and received do not match")
		return fmt.Errorf("data sent and received do not match")
	}

	if *allowReconnect {
		if e.reconnected {
			e.log.Info("Reconnect successful")
		} else {
			e.log.Error("Didn't handle reconnected connection")
			return fmt.Errorf("reconnect failed")
		}
	}

	return nil
}

func (e *EchoClient) runSendingLoop(ctx context.Context, conn net.Conn, dataSend *[]byte) error {
	additionalIterationsAfterTestDone := 0

	buff := make([]byte, bufSize)

	e.log.Info("Start sending data")

	iteration := 0
sendLoop:
	for {
		if iteration%100 == 0 {
			e.log.Info("Sending data", slog.Int("iteration", iteration))
		}

		rand.Read(buff)
		*dataSend = append(*dataSend, buff...)

		conn.SetWriteDeadline(time.Now().Add(operationTimeout))

		_, err := writeAll(conn, buff)
		if err != nil {
			e.log.Error("Error sending data", Err(err))
			return fmt.Errorf("sending data: %w", err)
		}

		select {
		case <-ctx.Done():
			if additionalIterationsAfterTestDone < iterationsAfterDone {
				additionalIterationsAfterTestDone++
				continue
			}
			break sendLoop
		case <-time.After(10 * time.Millisecond):
		}
		iteration++
	}

	err := conn.(*net.TCPConn).CloseWrite()
	if err != nil {
		e.log.Error("Error closing write side of connection", Err(err))
		return fmt.Errorf("closing write side of connection: %w", err)
	}

	return nil
}

func (e *EchoClient) runReceivingLoop(conn net.Conn, dataReceived *[]byte) error {
	buff := make([]byte, bufSize)

	iteration := 0
	for {
		conn.SetReadDeadline(time.Now().Add(operationTimeout))

		n, err := conn.Read(buff)
		if err != nil {
			if err != io.EOF {
				e.log.Error("Error reading response", Err(err))
				return fmt.Errorf("reading response: %w", err)
			}
			break
		}

		if iteration%100 == 0 {
			e.log.Info("Received data", slog.Int("bytes", n), slog.Int("iteration", iteration))
		}

		*dataReceived = append(*dataReceived, buff[:n]...)
		iteration++
	}

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
