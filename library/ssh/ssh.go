package ssh

import (
	"errors"

	"golang.org/x/crypto/ssh"
)

func New(config *ssh.ClientConfig) *SSH {
	return &SSH{
		Config: config,
	}
}

type SSH struct {
	Config  *ssh.ClientConfig
	Client  *ssh.Client
	Session *ssh.Session
}

func (s *SSH) Connect(addr string) error {
	client, err := ssh.Dial("tcp", addr, s.Config)
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

func (s *SSH) Close() error {
	if s.Session == nil {
		return nil
	}
	return s.Session.Close()
}
