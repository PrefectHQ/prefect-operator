package portforward

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"
)

// PortForwarder defines the interface for port-forwarding to a Kubernetes service
type PortForwarder interface {
	ForwardPorts(stopCh <-chan struct{}, readyCh chan<- struct{}) error
}

// KubectlPortForwarder implements PortForwarder using kubectl port-forward
type KubectlPortForwarder struct {
	Namespace  string
	Service    string
	LocalPort  int
	RemotePort int
}

// ForwardPorts starts a port-forwarding session using kubectl
func (p *KubectlPortForwarder) ForwardPorts(stopCh <-chan struct{}, readyCh chan<- struct{}) error {
	cmd := exec.Command("kubectl", "port-forward", "-n", p.Namespace, "svc/"+p.Service, fmt.Sprintf("%d:%d", p.LocalPort, p.RemotePort))
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start port-forwarding: %w", err)
	}

	// Wait for the port-forwarding to be ready
	ready := false
	for i := 0; i < 10; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", p.LocalPort))
		if err == nil {
			_ = resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(time.Second)
	}

	if !ready {
		_ = cmd.Process.Kill()
		return fmt.Errorf("port-forwarding failed to become ready")
	}

	close(readyCh)

	// Wait for stop signal
	<-stopCh
	_ = cmd.Process.Kill()
	return nil
}

// NewKubectlPortForwarder creates a new KubectlPortForwarder
func NewKubectlPortForwarder(namespace, service string, localPort, remotePort int) *KubectlPortForwarder {
	return &KubectlPortForwarder{
		Namespace:  namespace,
		Service:    service,
		LocalPort:  localPort,
		RemotePort: remotePort,
	}
}
