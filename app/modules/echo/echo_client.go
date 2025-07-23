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
	"sync"
	"time"

	"github.com/spf13/pflag"
)

var (
	serverAddress = pflag.String("echo-server-address", "echo-server.taler.svc.cluster.local", "Address of echo server")
	echoPort      = pflag.Int("echo-port", 8080, "Port of echo server")
	maxDataSizeMB = pflag.Int("max-data-size-mb", 3, "Maximum data transfered per connection in MB")
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
	parallelRequests int
	requestPerSecond int
	bufPool          *sync.Pool
}

func NewEchoClient(log *slog.Logger, availabilityZone, podName string) *EchoClient {
	return &EchoClient{
		log:              log,
		availabilityZone: availabilityZone,
		podName:          podName,
		bufPool: &sync.Pool{
			New: func() any {
				log.Info("Creating new buffer pool")
				return bytes.NewBuffer(make([]byte, 0, *maxDataSizeMB*MB))
			},
		},
	}
}

func (e *EchoClient) Run(ctx context.Context, requestNumberPerSecond int, parallelRequests int) error {
	e.log.Info("Starting client workers", slog.Int("parallel_requests", parallelRequests), slog.Int("request_per_second", requestNumberPerSecond))
	worker := func() {
		client := &http.Client{}
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			e.log.Info("Connecting to server.")
			bufSize := mathrand.Intn(*maxDataSizeMB-*minDataSizeMB) + *minDataSizeMB
			// Get buffer from pool
			buff := e.bufPool.Get().(*bytes.Buffer)
			// Reset and resize buffer
			buff.Reset()
			if buff.Cap() < bufSize*MB {
				buff = bytes.NewBuffer(make([]byte, 0, bufSize*MB))
			}
			// Fill buffer with random data
			tmp := make([]byte, bufSize*MB)
			rand.Read(tmp)
			buff.Write(tmp)

			e.log.Info("Sending data", slog.Int("buff-size", bufSize*MB))

			url := fmt.Sprintf("http://%s:%d/echo", *serverAddress, *echoPort)
			r, err := http.NewRequest("POST", url, buff)
			if err != nil {
				e.log.Error("Failed to create POST request.", Err(err))
				e.bufPool.Put(buff)
				continue
			}

			r.Header.Add("Content-Type", "text/plain")
			r.Header.Add(AvailabilityZoneHeader, e.availabilityZone)
			r.Header.Add(PodNameHeader, e.podName)

			resp, err := client.Do(r)
			if err != nil {
				e.log.Error("Error connecting to server", Err(err))
				e.bufPool.Put(buff)
				continue
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				e.log.Error("Error reading response", Err(err))
				e.bufPool.Put(buff)
				return
			}

			azLine := ""
			lines := bytes.SplitN(data, []byte{'\n'}, 2)
			if len(lines) > 0 {
				azLine = string(lines[0])
			}
			e.log.Info("Received data", slog.Int("bytes", len(data)), slog.Int("status_code", resp.StatusCode), slog.String("az", azLine))

			// Return buffer to pool
			e.bufPool.Put(buff)

			if resp.StatusCode == 200 {
				time.Sleep(time.Second / time.Duration(requestNumberPerSecond))
			}
		}
	}

	// Launch workers equal to parallelRequests
	for i := 0; i < parallelRequests; i++ {
		go worker()
	}

	<-ctx.Done()
	return ctx.Err()

}

func Err(err error) slog.Attr {
	return slog.String("err", err.Error())
}
