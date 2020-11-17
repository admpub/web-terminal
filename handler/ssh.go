package handler

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/admpub/web-terminal/config"
	sshx "github.com/admpub/web-terminal/library/ssh"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
)

func NewSSHConfig(ws *websocket.Conn, account *sshx.AccountConfig) (*ssh.ClientConfig, error) {
	return sshx.NewSSHConfig(bufio.NewReader(ws), ws, account)
}

func NewHostConfig(ws *websocket.Conn, account *sshx.AccountConfig, host string, port int) (*sshx.HostConfig, error) {
	clientConfig, err := NewSSHConfig(ws, account)
	if err != nil {
		return nil, err
	}
	return &sshx.HostConfig{
		ClientConfig: clientConfig,
		Host:         host,
		Port:         port,
	}, err
}

var (
	SSHAccountContextKey         = struct{}{}
	SSHConfigContextKey          = struct{}{}
	SSHEndHostConfigContextKey   = struct{}{}
	SSHJumpHostConfigsContextKey = struct{}{}
)

func getSSHAccount(ctx *Context) *sshx.AccountConfig {
	account, ok := ctx.Request().Context().Value(SSHAccountContextKey).(*sshx.AccountConfig)
	if ok {
		return account
	}
	user := ParamGet(ctx, "user")
	pwd := ParamGet(ctx, "password")
	charset := ParamGet(ctx, "charset")
	// Dial code is taken from the ssh package example
	account = &sshx.AccountConfig{
		User:     user,
		Password: pwd,
		Charset:  charset,
	}
	if privKey := ParamGet(ctx, "privateKey"); len(privKey) > 0 {
		account.PrivateKey = []byte(privKey)
	}
	if passphrase := ParamGet(ctx, "passphrase"); len(passphrase) > 0 {
		account.Passphrase = []byte(passphrase)
	}
	return account
}

func getHostConfig(ctx *Context) (*sshx.HostConfig, error) {
	hostConfig, ok := ctx.Request().Context().Value(SSHAccountContextKey).(*sshx.HostConfig)
	if ok {
		return hostConfig, nil
	}
	hostname := ParamGet(ctx, "hostname")
	port := ParamGet(ctx, "port")
	if 0 == len(port) {
		port = "22"
	}
	portN, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	account := getSSHAccount(ctx)
	hostConfig, err = NewHostConfig(ctx.Conn, account, hostname, portN)
	if err != nil {
		return hostConfig, err
	}
	hostConfig.SetAccount(account)
	return hostConfig, err
}

func SSHShell(ctx *Context) error {
	var dumpOut, dumpIn io.WriteCloser
	defer func() {
		ctx.Close()
		if nil != dumpOut {
			dumpOut.Close()
		}
		if nil != dumpIn {
			dumpIn.Close()
		}
	}()
	columns := toInt(ParamGet(ctx, "columns"), 120)
	rows := toInt(ParamGet(ctx, "rows"), 80)
	debug := config.Default.Debug
	if "true" == strings.ToLower(ParamGet(ctx, "debug")) {
		debug = true
	}

	if ctx.Config.End == nil {
		hostConfig, err := getHostConfig(ctx)
		if err != nil {
			return fmt.Errorf("Failed to dial:: %w", err)
		}
		ctx.Config.SetEnd(hostConfig)
	}
	sshClient := sshx.New(ctx.Config)
	err := sshClient.Connect()
	if err != nil {
		return err
	}
	session := sshClient.Session
	defer sshClient.Close()

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // enable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	// Request pseudo terminal
	if err = session.RequestPty("xterm", rows, columns, modes); err != nil {
		return fmt.Errorf("request for pseudo terminal failed: %w", err)
	}
	ws := ctx.Conn
	hostConfig := ctx.Config.End
	combinedOut := decodeBy(hostConfig.Account.Charset, ws)
	if debug {
		dumpOut, err = os.OpenFile(config.Default.LogDir+hostConfig.Host+".dump_ssh_out.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if nil == err {
			combinedOut = io.MultiWriter(dumpOut, decodeBy(hostConfig.Account.Charset, ws))
		}

		dumpIn, err = os.OpenFile(config.Default.LogDir+hostConfig.Host+".dump_ssh_in.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if nil != err {
			dumpIn = nil
		}
	}

	session.Stdout = combinedOut
	session.Stderr = combinedOut
	session.Stdin = warp(ws, dumpIn)
	if err := session.Shell(); nil != err {
		return fmt.Errorf("Unable to execute command: %w", err)
	}
	if err := session.Wait(); nil != err {
		return fmt.Errorf("Unable to execute command: %w", err)
	}
	return nil
}

func SSHExec(ctx *Context) error {
	var dumpOut, dumpIn io.WriteCloser
	defer func() {
		ctx.Close()
		if nil != dumpOut {
			dumpOut.Close()
		}
		if nil != dumpIn {
			dumpIn.Close()
		}
	}()
	debug := config.Default.Debug
	if "true" == strings.ToLower(ParamGet(ctx, "debug")) {
		debug = true
	}
	cmd := ParamGet(ctx, "cmd")
	cmdAlias := ParamGet(ctx, "dump_file")
	if 0 == len(cmdAlias) {
		cmdAlias = strings.Replace(cmd, " ", "_", -1)
	}
	ws := ctx.Conn
	if ctx.Config.End == nil {
		hostConfig, err := getHostConfig(ctx)
		if err != nil {
			return fmt.Errorf("Failed to dial: %w", err)
		}
		ctx.Config.SetEnd(hostConfig)
	}
	sshClient := sshx.New(ctx.Config)
	err := sshClient.Connect()
	if err != nil {
		return err
	}
	session := sshClient.Session
	defer sshClient.Close()
	hostConfig := ctx.Config.End
	combinedOut := decodeBy(hostConfig.Account.Charset, ws)
	if debug {
		dumpOut, err = os.OpenFile(config.Default.LogDir+hostConfig.Host+"_"+cmdAlias+".dump_ssh_out.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if nil == err {
			fmt.Println("log to file", config.Default.LogDir+hostConfig.Host+"_"+cmdAlias+".dump_ssh_out.txt")
			combinedOut = io.MultiWriter(dumpOut, decodeBy(hostConfig.Account.Charset, ws))
		} else {
			fmt.Println("failed to open log file,", err)
		}

		dumpIn, err = os.OpenFile(config.Default.LogDir+hostConfig.Host+"_"+cmdAlias+".dump_ssh_in.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if nil != err {
			dumpIn = nil
			fmt.Println("failed to open log file,", err)
		} else {
			fmt.Println("log to file", config.Default.LogDir+hostConfig.Host+"_"+cmdAlias+".dump_ssh_in.txt")
		}
	}

	session.Stdout = combinedOut
	session.Stderr = combinedOut
	session.Stdin = warp(ws, dumpIn)

	if err := session.Start(cmd); nil != err {
		return fmt.Errorf("Unable to execute command: %w", err)
	}
	if err := session.Wait(); nil != err {
		return fmt.Errorf("Unable to execute command: %w", err)
	}
	fmt.Println("exec ok")
	return nil
}

func linuxSSH(ws *websocket.Conn, args []string, charset, wd string, timeout time.Duration) {
	log.Println("begin to execute ssh:", args)

	// [ssh -batch -pw 8498b2c7 root@192.168.1.18 -f /var/lib/tpt/etc/scripts/abc.sh]
	pw := config.Default.Password
	idFile := config.Default.IDFile

	if len(config.Default.SHFile) > 0 {
		bs, err := ioutil.ReadFile(config.Default.SHFile)
		if err != nil {
			io.WriteString(ws, "parse arguments error: command is missing")
			return
		}
		bs = bytes.TrimSpace(bs)
		if len(bs) == 0 {
			io.WriteString(ws, args[2]+" is empty")
			return
		}

		args = []string{args[0], string(bs)}
	}

	if len(idFile) > 0 {
		args = append([]string{"-i", idFile, "-o", "StrictHostKeyChecking=no"}, args...)
	} else {
		args = append([]string{"-o", "StrictHostKeyChecking=no"}, args...)
	}

	output := decodeBy(charset, ws)

	var cmd *exec.Cmd
	if len(pw) > 0 {
		cmd = exec.Command("sshpass", append([]string{"-p", pw, "ssh"}, args...)...)
	} else {
		cmd = exec.Command("ssh", args...)
	}
	if len(wd) > 0 {
		cmd.Dir = wd
	}

	cmd.Stdin = ws
	cmd.Stderr = output
	cmd.Stdout = output

	log.Println(cmd.Path, cmd.Args)

	if err := cmd.Start(); err != nil {
		io.WriteString(ws, err.Error())
		return
	}

	go func() {
		defer recover()

		cmd.Process.Wait()
		ws.Close()
	}()

	timer := time.AfterFunc(timeout, func() {
		defer recover()
		cmd.Process.Kill()
	})

	if err := cmd.Wait(); err != nil {
		io.WriteString(ws, err.Error())
	}
	timer.Stop()
	ws.Close()
}
