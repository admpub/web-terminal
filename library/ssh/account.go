package ssh

import (
	"io"
	"runtime"

	"golang.org/x/crypto/ssh"
)

type AccountConfig struct {
	User       string
	Password   string
	PrivateKey []byte
	Passphrase []byte
	Charset    string
}

func (a *AccountConfig) SetDefault() *AccountConfig {
	if 0 == len(a.Charset) {
		if "windows" == runtime.GOOS {
			a.Charset = "GB18030"
		} else {
			a.Charset = "UTF-8"
		}
	}
	return a
}

func (a *AccountConfig) BuildClientConfig(reader io.Reader, writer io.Writer) (*ssh.ClientConfig, error) {
	return NewSSHConfig(reader, writer, a)
}
