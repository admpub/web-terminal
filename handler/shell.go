package handler

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/admpub/web-terminal/config"
	"github.com/admpub/web-terminal/library/utils"
	"github.com/fd/go-shellwords/shellwords"
	"golang.org/x/net/websocket"
)

func ExecShell(ctx *Context) error {
	defer ctx.Close()
	queryParams := ctx.Request().URL.Query()
	wd := ParamGet(ctx, "wd")
	charset := ParamGet(ctx, "charset")
	pa := ParamGet(ctx, "exec")
	timeout := ParamGet(ctx, "timeout")
	stdin := ParamGet(ctx, "stdin")

	args := make([]string, 0, 10)
	for i := 0; i < 1000; i++ {
		arguments, ok := queryParams["arg"+strconv.FormatInt(int64(i), 10)]
		if !ok {
			break
		}
		args = append(args, arguments...)
	}

	return execShell(ctx.Conn, pa, args, charset, wd, stdin, timeout)
}

func ExecShell2(ctx *Context) error {
	defer ctx.Close()
	wd := ParamGet(ctx, "wd")
	charset := ParamGet(ctx, "charset")
	pa := ParamGet(ctx, "exec")
	timeout := ParamGet(ctx, "timeout")
	stdin := ParamGet(ctx, "stdin")

	ss, e := shellwords.Split(pa)
	if nil != e {
		return fmt.Errorf("命令格式不正确：%w", e)
	}
	pa = ss[0]
	args := ss[1:]

	return execShell(ctx.Conn, pa, args, charset, wd, stdin, timeout)
}

func execShell(ws *websocket.Conn, pa string, args []string, charset, wd, stdin, timeoutStr string) error {
	//ctx := NewContext(ws)
	charset = fixCharset(charset)

	timeout := 10 * time.Minute
	if len(timeoutStr) > 0 {
		if t, e := time.ParseDuration(timeoutStr); nil == e {
			timeout = t
		}
	}

	queryParams := ws.Request().URL.Query()
	if _, ok := queryParams["file"]; ok {
		fileContent := queryParams.Get("file")
		f, e := os.CreateTemp(os.TempDir(), "run")
		if nil != e {
			return fmt.Errorf("生成临时文件失败：%w", e)
		}

		filename := f.Name()
		defer func() {
			f.Close()
			os.Remove(filename)
		}()

		_, e = io.WriteString(f, fileContent)
		if nil != e {
			return fmt.Errorf("写临时文件失败：%w", e)
		}
		f.Close()

		args = append(args, filename)
	}

	if pa == "ssh" && runtime.GOOS != "windows" {
		linuxSSH(ws, args, charset, wd, timeout)
		return nil
	}

	if strings.HasPrefix(pa, "snmp") {
		args = utils.AddMibDir(args)
	} else if pa == "tpt" || pa == "tpt.exe" {
		if "windows" == runtime.GOOS {
			args = append([]string{"-gbk=true"}, args...)
		}
	}

	if c, ok := commands[pa]; ok {
		pa = c
	} else {
		if newPa, ok := utils.LookPath(config.ExecutableFolder, pa); ok {
			pa = newPa
		}
	}

	isConnectionAbandoned := false
	output := decodeBy(charset, ws)
	if pp := strings.ToLower(pa); strings.HasSuffix(pp, "plink.exe") || strings.HasSuffix(pp, "plink") {
		output = matchBy(output, "Connection abandoned.", func() {
			isConnectionAbandoned = true
		})
	}

	cmd := exec.Command(pa, args...)
	if len(wd) > 0 {
		cmd.Dir = wd
	}
	if stdin == "on" {
		cmd.Stdin = ws
	}
	cmd.Stderr = output
	cmd.Stdout = output

	log.Println(cmd.Path, cmd.Args)

	if err := cmd.Start(); err != nil {

		if !os.IsPermission(err) || runtime.GOOS == "windows" {
			return err
		}

		newArgs := append(make([]string, len(args)+1))
		newArgs[0] = pa
		copy(newArgs[1:], args)
		cmd = exec.Command(config.Default.SHExecute, newArgs...)
		if len(wd) > 0 {
			cmd.Dir = wd
		}
		cmd.Stdin = ws
		cmd.Stderr = output
		cmd.Stdout = output

		log.Println(cmd.Path, cmd.Args)
		if err := cmd.Start(); err != nil {
			return err
		}
	}

	timer := time.AfterFunc(timeout, func() {
		defer recover()
		cmd.Process.Kill()
	})

	if stdin == "on" {
		if state, err := cmd.Process.Wait(); err != nil {
			io.WriteString(ws, err.Error())
		} else if state != nil && !state.Success() {
			io.WriteString(ws, state.String())
		}
	} else {
		if err := cmd.Wait(); err != nil {
			io.WriteString(ws, err.Error())
		}
	}
	timer.Stop()
	if err := ws.Close(); err != nil {
		log.Println(err)
	}

	if isConnectionAbandoned {
		utils.SaveSessionKey(pa, args, wd)
	}
	return nil
}
