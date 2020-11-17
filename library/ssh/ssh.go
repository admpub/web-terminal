package ssh

import (
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	defaultTimeout = 10 * time.Second
	defaultPort    = 22
)

func New(config *Config) *SSH {
	return &SSH{
		Config: config,
	}
}

type SSH struct {
	Config  *Config
	Client  *ssh.Client
	Session *ssh.Session
	Timeout time.Duration
}

func (s *SSH) Connect() error {
	client, err := s.buildClient()
	if err != nil {
		return errors.New("Failed to dial: " + err.Error())
	}
	s.Client = client

	session, err := client.NewSession()
	if err != nil {
		return errors.New("Failed to create session: " + err.Error())
	}
	s.Session = session

	return nil
}

// buildClient builds the *ssh.Client connection via the jump
// host to the end host. ATM only one jump host is supported
func (s *SSH) buildClient() (*ssh.Client, error) {
	port := defaultPort
	if s.Config.End.Port > 0 {
		port = s.Config.End.Port
	}
	endHostAddress := fmt.Sprintf(`%s:%d`, s.Config.End.Host, port)
	if len(s.Config.Jumps) > 0 { //TODO atm only one jump host is supported
		jumpConfg := s.Config.Jumps[0]
		port := defaultPort
		if jumpConfg.Port > 0 {
			port = jumpConfg.Port
		}
		jumpHostAddress := fmt.Sprintf(`%s:%d`, jumpConfg.Host, port)
		jumpHostClient, err := ssh.Dial("tcp", jumpHostAddress, jumpConfg.ClientConfig)
		if err != nil {
			return nil, fmt.Errorf("ssh.Dial to jump host failed: %s", err)
		}
		jumpHostConn, err := s.dialNextJump(jumpHostClient, endHostAddress)
		if err != nil {
			return nil, fmt.Errorf("ssh.Dial from jump to jump host failed: %s", err)
		}

		ncc, chans, reqs, err := ssh.NewClientConn(jumpHostConn, endHostAddress, s.Config.End.ClientConfig)
		if err != nil {
			jumpHostConn.Close()
			return nil, fmt.Errorf("Failed to create ssh client to end host: %s", err)
		}

		return ssh.NewClient(ncc, chans, reqs), nil
	}

	endHostClient, err := ssh.Dial("tcp", endHostAddress, s.Config.End.ClientConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh.Dial directly to end host failed: %s", err)
	}

	return endHostClient, nil
}

// dialNextJump dials the next jump host in the chain
func (s *SSH) dialNextJump(jumpHostClient *ssh.Client, nextJumpAddress string) (net.Conn, error) {
	// NOTE: no timeout param in ssh.Dial: https://github.com/golang/go/issues/20288
	// implement it by hand
	var (
		jumpHostConn net.Conn
		err          error
	)
	connChan := make(chan net.Conn)
	go func() {
		jumpHostConn, err = jumpHostClient.Dial("tcp", nextJumpAddress)
		if err != nil {
			return
		}
		connChan <- jumpHostConn
	}()
	connectionTimeout := s.Timeout
	if connectionTimeout <= 0 {
		connectionTimeout = defaultTimeout
	}
	select {
	case jumpHostConnSel := <-connChan:
		jumpHostConn = jumpHostConnSel
	case <-time.After(connectionTimeout):
		return nil, fmt.Errorf("ssh.Dial from jump host to next jump failed after timeout")
	}

	return jumpHostConn, err
}

func (s *SSH) Close() error {
	if s.Session == nil {
		return nil
	}
	return s.Session.Close()
}
