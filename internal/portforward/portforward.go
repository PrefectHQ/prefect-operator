// Package portforward runs kubectl port-forward to a Kubernetes Service, so
// that an operator running outside the cluster can reach an in-cluster
// Prefect server.
package portforward

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// readyTimeout is how long Start waits for kubectl to report its local port
// and for the server to answer through the forward.
const readyTimeout = 10 * time.Second

// forwardingLine matches the line kubectl prints once it listens, such as
// "Forwarding from 127.0.0.1:53021 -> 4200".
var forwardingLine = regexp.MustCompile(`^Forwarding from 127\.0\.0\.1:(\d+) -> `)

// Forward is a running kubectl port-forward.
type Forward struct {
	// LocalPort is the port on 127.0.0.1 that forwards to the Service.
	LocalPort int

	cancel context.CancelFunc
	done   chan struct{}
	stderr *lockedBuffer
	err    error
}

// Start runs kubectl port-forward from 127.0.0.1 to remotePort of the Service
// named service in namespace. kubectl picks a free local port, so concurrent
// forwards never share one. Start returns once an HTTP request through the
// forward gets a response. The forward stops when ctx ends or Close is called.
// namespace must not be empty: kubectl would use its context's namespace.
func Start(ctx context.Context, namespace, service string, remotePort int) (*Forward, error) {
	if namespace == "" {
		return nil, errors.New("port-forward needs a namespace")
	}

	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, "kubectl", "port-forward",
		"--namespace", namespace, "--address", "127.0.0.1",
		"svc/"+service, fmt.Sprintf(":%d", remotePort))
	// kubectl prints a line for every connection, so stdout is read until
	// kubectl exits. WaitDelay stops Wait from blocking on the pipe if a
	// child process holds it open after kubectl exits.
	stdout, stdoutWriter := io.Pipe()
	f := &Forward{cancel: cancel, done: make(chan struct{}), stderr: &lockedBuffer{}}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = f.stderr
	cmd.WaitDelay = time.Second

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("starting kubectl port-forward: %w", err)
	}
	go func() {
		f.err = cmd.Wait()
		_ = stdoutWriter.Close()
		close(f.done)
	}()

	ports := make(chan int, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if m := forwardingLine.FindStringSubmatch(scanner.Text()); m != nil {
				if port, err := strconv.Atoi(m[1]); err == nil {
					select {
					case ports <- port:
					default:
					}
				}
			}
		}
		_, _ = io.Copy(io.Discard, stdout)
	}()

	deadline := time.NewTimer(readyTimeout)
	defer deadline.Stop()
	select {
	case f.LocalPort = <-ports:
	case <-f.done:
		return nil, f.exitError(namespace, service)
	case <-deadline.C:
		f.Close()
		return nil, fmt.Errorf("kubectl port-forward to svc/%s in namespace %s did not report a local port within %s",
			service, namespace, readyTimeout)
	}

	if err := f.waitUntilConnected(deadline.C); err != nil {
		f.Close()
		return nil, fmt.Errorf("port-forward to svc/%s in namespace %s: %w", service, namespace, err)
	}
	return f, nil
}

// waitUntilConnected sends requests through the forward until one gets any
// HTTP response, kubectl exits, or deadline fires.
func (f *Forward) waitUntilConnected(deadline <-chan time.Time) error {
	probe := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/health", f.LocalPort)
	for {
		resp, err := probe.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		select {
		case <-f.done:
			return fmt.Errorf("kubectl exited: %s", f.stderr.lastLine())
		case <-deadline:
			return fmt.Errorf("no response within %s: %w", readyTimeout, err)
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (f *Forward) exitError(namespace, service string) error {
	return fmt.Errorf("kubectl port-forward to svc/%s in namespace %s exited: %v: %s",
		service, namespace, f.err, f.stderr.lastLine())
}

// Done returns a channel that is closed when kubectl exits.
func (f *Forward) Done() <-chan struct{} {
	return f.done
}

// Close stops kubectl and waits for it to exit.
func (f *Forward) Close() {
	f.cancel()
	<-f.done
}

// stderrLimit is how many bytes of kubectl's stderr lockedBuffer keeps.
const stderrLimit = 4096

// lockedBuffer keeps the last stderrLimit bytes of kubectl's stderr. exec
// writes to it from its own goroutine while Start may read it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.buf.Write(p)
	if extra := b.buf.Len() - stderrLimit; extra > 0 {
		b.buf.Next(extra)
	}
	return n, err
}

func (b *lockedBuffer) lastLine() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	lines := bytes.Split(bytes.TrimSpace(b.buf.Bytes()), []byte("\n"))
	return string(lines[len(lines)-1])
}
