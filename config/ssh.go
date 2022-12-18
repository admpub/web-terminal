package config

import (
	"golang.org/x/crypto/ssh"
)

func NewHostConfig(cfg *ssh.ClientConfig, host string, port int) *HostConfig {
	return &HostConfig{ClientConfig: cfg, Host: host, Port: port}
}

type HostConfig struct {
	*ssh.ClientConfig
	Host    string
	Port    int
	Account *AccountConfig
}

func (c *HostConfig) SetAccount(account *AccountConfig) *HostConfig {
	c.Account = account
	return c
}

func NewSSHConfig() *SSHConfig {
	return &SSHConfig{
		Transform: NewTransformConfig(),
	}
}

type SSHConfig struct {
	End       *HostConfig
	Jumps     []*HostConfig
	Transform *TransformConfig
}

func (c *SSHConfig) SetEnd(endHostConfig *HostConfig) *SSHConfig {
	c.End = endHostConfig
	return c
}

func (c *SSHConfig) AddJump(jumpHostConfig *HostConfig) *SSHConfig {
	c.Jumps = append(c.Jumps, jumpHostConfig)
	return c
}
