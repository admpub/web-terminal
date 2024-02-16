package config_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/admpub/web-terminal/config"
	"github.com/admpub/web-terminal/library/ssh"
)

func _TestSSH(t *testing.T) {
	clientCfg, err := config.NewSSHStandard(os.Stdin, os.Stdout, &config.AccountConfig{
		User:     `hank`,
		Password: ``,
	})
	if err != nil {
		panic(err)
	}
	hostCfg := config.NewHostConfig(clientCfg, `s2.admpub.com`, 22)
	cfg := config.NewSSHConfig(hostCfg)
	client := ssh.New(cfg)
	err = client.Connect()
	if err != nil {
		panic(err)
	}
	defer client.Close()
	client.Session.Stdout = os.Stdout
	client.Session.Stderr = os.Stderr
	err = client.RunCmd(`ls -alh .`)
	if err != nil {
		panic(err)
	}
	cmdPipe := bytes.NewBuffer(nil)
	cmdPipe.WriteString(`ls -l .` + "\n")
	cmdPipe.WriteString(`date` + "\n")
	err = client.RunCmds(cmdPipe)
	if err != nil {
		panic(err)
	}
}
