package handler

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/admpub/web-terminal/config"
	sshx "github.com/admpub/web-terminal/library/ssh"
	"golang.org/x/net/websocket"
)

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
		hostConfig, err := ctx.GetHostConfig()
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
	onInit := func() error {
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
		return nil
	}
	err = sshClient.StartShellWithCallback(onInit, rows, columns)
	return err
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
		hostConfig, err := ctx.GetHostConfig()
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
		bs, err := os.ReadFile(config.Default.SHFile)
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
