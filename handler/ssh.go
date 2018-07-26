package handler

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/admpub/web-terminal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
)

func NewSSHConfig(ws *websocket.Conn, user, pwd string) *ssh.ClientConfig {
	passwordCount := 0
	emptyInteractiveCount := 0
	reader := bufio.NewReader(ws)
	// Dial code is taken from the ssh package example
	sshConfig := &ssh.ClientConfig{
		Config:          ssh.Config{Ciphers: supportedCiphers},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		User:            user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pwd),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
				if len(questions) == 0 {
					emptyInteractiveCount++
					if emptyInteractiveCount++; emptyInteractiveCount > 50 {
						return nil, errors.New("interactive count is too much")
					}
					return []string{}, nil
				}
				for _, question := range questions {
					io.WriteString(ws, question)

					switch strings.ToLower(strings.TrimSpace(question)) {
					case "password:", "password as":
						passwordCount++
						if passwordCount == 1 {
							answers = append(answers, pwd)
							break
						}
						fallthrough
					default:
						line, _, e := reader.ReadLine()
						if nil != e {
							return nil, e
						}
						answers = append(answers, string(line))
					}
				}
				return answers, nil
			}),
		},
	}
	return sshConfig
}

func SSHShell(ws *websocket.Conn) {
	var dumpOut, dumpIn io.WriteCloser
	defer func() {
		ws.Close()
		if nil != dumpOut {
			dumpOut.Close()
		}
		if nil != dumpIn {
			dumpIn.Close()
		}
	}()

	hostname := ParamGet(ws, "hostname")
	port := ParamGet(ws, "port")
	if 0 == len(port) {
		port = "22"
	}
	user := ParamGet(ws, "user")
	pwd := ParamGet(ws, "password")
	columns := toInt(ParamGet(ws, "columns"), 120)
	rows := toInt(ParamGet(ws, "rows"), 80)
	debug := config.Default.Debug
	if "true" == strings.ToLower(ParamGet(ws, "debug")) {
		debug = true
	}

	// Dial code is taken from the ssh package example
	sshConfig := NewSSHConfig(ws, user, pwd)
	client, err := ssh.Dial("tcp", hostname+":"+port, sshConfig)
	if err != nil {
		logString(ws, "Failed to dial: "+err.Error())
		return
	}

	session, err := client.NewSession()
	if err != nil {
		logString(ws, "Failed to create session: "+err.Error())
		return
	}
	defer session.Close()

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	// Request pseudo terminal
	if err = session.RequestPty("xterm", rows, columns, modes); err != nil {
		logString(ws, "request for pseudo terminal failed:"+err.Error())
		return
	}

	var combinedOut io.Writer = ws
	if debug {
		dumpOut, err = os.OpenFile(config.Default.LogDir+hostname+".dump_ssh_out.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if nil == err {
			combinedOut = io.MultiWriter(dumpOut, ws)
		}

		dumpIn, err = os.OpenFile(config.Default.LogDir+hostname+".dump_ssh_in.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if nil != err {
			dumpIn = nil
		}
	}

	session.Stdout = combinedOut
	session.Stderr = combinedOut
	session.Stdin = warp(ws, dumpIn)
	if err := session.Shell(); nil != err {
		logString(ws, "Unable to execute command:"+err.Error())
		return
	}
	if err := session.Wait(); nil != err {
		logString(ws, "Unable to execute command:"+err.Error())
	}
}

func SSHExec(ws *websocket.Conn) {
	var dumpOut, dumpIn io.WriteCloser
	defer func() {
		ws.Close()
		if nil != dumpOut {
			dumpOut.Close()
		}
		if nil != dumpIn {
			dumpIn.Close()
		}
	}()

	hostname := ParamGet(ws, "hostname")
	port := ParamGet(ws, "port")
	if len(port) == 0 {
		port = "22"
	}
	user := ParamGet(ws, "user")
	pwd := ParamGet(ws, "password")
	debug := config.Default.Debug
	if "true" == strings.ToLower(ParamGet(ws, "debug")) {
		debug = true
	}

	cmd := ParamGet(ws, "cmd")
	cmdAlias := ParamGet(ws, "dump_file")
	if "" == cmdAlias {
		cmdAlias = strings.Replace(cmd, " ", "_", -1)
	}

	// Dial code is taken from the ssh package example
	sshConfig := NewSSHConfig(ws, user, pwd)
	client, err := ssh.Dial("tcp", hostname+":"+port, sshConfig)
	if err != nil {
		logString(ws, "Failed to dial: "+err.Error())
		return
	}

	session, err := client.NewSession()
	if err != nil {
		logString(ws, "Failed to create session: "+err.Error())
		return
	}
	defer session.Close()

	var combinedOut io.Writer = ws
	if debug {
		dumpOut, err = os.OpenFile(config.Default.LogDir+hostname+"_"+cmdAlias+".dump_ssh_out.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if nil == err {
			fmt.Println("log to file", config.Default.LogDir+hostname+"_"+cmdAlias+".dump_ssh_out.txt")
			combinedOut = io.MultiWriter(dumpOut, ws)
		} else {
			fmt.Println("failed to open log file,", err)
		}

		dumpIn, err = os.OpenFile(config.Default.LogDir+hostname+"_"+cmdAlias+".dump_ssh_in.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if nil != err {
			dumpIn = nil
			fmt.Println("failed to open log file,", err)
		} else {
			fmt.Println("log to file", config.Default.LogDir+hostname+"_"+cmdAlias+".dump_ssh_in.txt")
		}
	}

	session.Stdout = combinedOut
	session.Stderr = combinedOut
	session.Stdin = warp(ws, dumpIn)

	if err := session.Start(cmd); nil != err {
		logString(combinedOut, "Unable to execute command:"+err.Error())
		return
	}
	if err := session.Wait(); nil != err {
		logString(combinedOut, "Unable to execute command:"+err.Error())
		return
	}
	fmt.Println("exec ok")
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
