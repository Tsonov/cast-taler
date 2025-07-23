package echo

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	mathrand "math/rand"
	"net/http"
	"time"

	"github.com/spf13/pflag"
)

var (
	serverAddress = pflag.String("echo-server-address", "echo-server", "Address of echo server")
	echoPort      = pflag.Int("echo-port", 8080, "Port of echo server")
	maxDataSizeMB = pflag.Int("max-data-size-mb", 5, "Maximum data transfered per connection in MB")
	minDataSizeMB = pflag.Int("min-data-size-mb", 1, "Minimum data transfered per connection in MB")
)

const (
	MB               = 1024 * 1024
	operationTimeout = 20 * time.Second
)

type EchoClient struct {
	log              *slog.Logger
	availabilityZone string
	podName          string
}

func NewEchoClient(log *slog.Logger, availabilityZone, podName string) *EchoClient {
	return &EchoClient{
		log:              log,
		availabilityZone: availabilityZone,
		podName:          podName,
	}
}

func (e *EchoClient) Run(ctx context.Context, requestNumberPerSecond int) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		e.log.Info("Connecting to server.")
		bufSize := mathrand.Intn(*maxDataSizeMB-*minDataSizeMB) + *minDataSizeMB
		buff := make([]byte, bufSize*MB)

		e.log.Info("Sending data", slog.Int("buff-size", bufSize*MB))
		rand.Read(buff)

		url := fmt.Sprintf("http://%s:%d/echo", *serverAddress, *echoPort)
		r, err := http.NewRequest("POST", url, bytes.NewBuffer(buff))
		if err != nil {
			e.log.Error("Failed to create POST request.", Err(err))
			return fmt.Errorf("create POST request: %w", err)

		}

		r.Header.Add("Content-Type", "text/plain")
		r.Header.Add(AvailabilityZoneHeader, e.availabilityZone)
		r.Header.Add(PodNameHeader, e.podName)

		client := &http.Client{}
		resp, err := client.Do(r)
		if err != nil {
			e.log.Error("Error connecting to server", Err(err))
			return fmt.Errorf("connecting to server: %w", err)
		}
		defer resp.Body.Close()

		e.log.Info("Receiving data")
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			e.log.Error("Error reading response", Err(err))
			return fmt.Errorf("reading response: %w", err)
		}

		e.log.Info("Received data", slog.Int("bytes", len(data)), slog.Int("status_code", resp.StatusCode))

		time.Sleep(time.Second / time.Duration(requestNumberPerSecond))
	}

}

func Err(err error) slog.Attr {
	return slog.String("err", err.Error())
}
