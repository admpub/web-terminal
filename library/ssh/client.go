package ssh

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/admpub/web-terminal/config"
	"golang.org/x/crypto/ssh"
)

func NewClient(ctx context.Context, cfg *config.SSHConfig, timeout time.Duration) (*ssh.Client, error) {
	return buildClient(ctx, cfg, timeout)
}

// buildClient builds the *ssh.Client connection via the jump
// host to the end host. ATM only one jump host is supported
func buildClient(ctx context.Context, cfg *config.SSHConfig, timeout time.Duration) (*ssh.Client, error) {
	port := defaultPort
	if cfg.End.Port > 0 {
		port = cfg.End.Port
	}
	endHostAddress := fmt.Sprintf(`%s:%d`, cfg.End.Host, port)
	if len(cfg.Jumps) > 0 { //TODO atm only one jump host is supported
		jumpConfg := cfg.Jumps[0]
		port := defaultPort
		if jumpConfg.Port > 0 {
			port = jumpConfg.Port
		}
		jumpHostAddress := fmt.Sprintf(`%s:%d`, jumpConfg.Host, port)
		jumpHostClient, err := ssh.Dial("tcp", jumpHostAddress, jumpConfg.ClientConfig)
		if err != nil {
			return nil, fmt.Errorf("ssh.Dial to jump host failed: %w", err)
		}
		jumpHostConn, err := dialNextJump(ctx, jumpHostClient, endHostAddress, timeout)
		if err != nil {
			return nil, fmt.Errorf("ssh.Dial from jump to jump host(%q) failed: %w", endHostAddress, err)
		}

		ncc, chans, reqs, err := ssh.NewClientConn(jumpHostConn, endHostAddress, cfg.End.ClientConfig)
		if err != nil {
			jumpHostConn.Close()
			return nil, fmt.Errorf("failed to create ssh client to end host(%q): %w", endHostAddress, err)
		}

		return ssh.NewClient(ncc, chans, reqs), nil
	}

	endHostClient, err := ssh.Dial("tcp", endHostAddress, cfg.End.ClientConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh.Dial directly to end host(%q) failed: %w", endHostAddress, err)
	}

	return endHostClient, nil
}

// dialNextJump dials the next jump host in the chain
func dialNextJump(ctx context.Context, jumpHostClient *ssh.Client, nextJumpAddress string, timeout time.Duration) (net.Conn, error) {
	connectionTimeout := timeout
	if connectionTimeout <= 0 {
		connectionTimeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()
	jumpHostConn, err := jumpHostClient.DialContext(ctx, "tcp", nextJumpAddress)
	return jumpHostConn, err
}
