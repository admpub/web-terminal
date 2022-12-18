package handler

import (
	"io"
	"net/http"
	"runtime"
	"strings"

	"github.com/admpub/web-terminal/library/utils"

	"github.com/admpub/web-terminal/config"
	"golang.org/x/net/websocket"
)

var (
	commands     = map[string]string{}
	findCommands = []string{
		"snmpget",
		"snmpgetnext",
		"snmpdf",
		"snmpbulkget",
		"snmpbulkwalk",
		"snmpdelta",
		"snmpnetstat",
		"snmpset",
		"snmpstatus",
		"snmptable",
		"snmptest",
		"snmptools",
		"snmptranslate",
		"snmptrap",
		"snmpusm",
		"snmpvacm",
		"snmpwalk",
		"wshell",
	}

	//ParamGet 获取参数值
	ParamGet = func(ctx *Context, name string) string {
		return ctx.Request().URL.Query().Get(name)
	}
)

func init() {
	fillCommands(config.ExecutableFolder)
}

func warp(dst io.ReadCloser, dump io.Writer) io.ReadCloser {
	return utils.Warp(dst, dump)
}

func decodeBy(charset string, dst io.Writer) io.Writer {
	return config.DecodeBy(charset, dst)
}

func matchBy(dst io.Writer, excepted string, cb func()) io.Writer {
	return utils.MatchBy(dst, excepted, cb)
}

func toInt(s string, v int) int {
	return utils.ToInt(s, v)
}

func logString(ws io.Writer, msg string) {
	utils.LogString(ws, msg)
}

func fixCharset(charset string) string {

	if 0 == len(charset) {
		if "windows" == runtime.GOOS {
			charset = "GB18030"
		} else {
			charset = "UTF-8"
		}
	}

	return charset
}

func fillCommands(executableFolder string) {
	for _, nm := range findCommands {
		if pa, ok := utils.LookPath(executableFolder, nm); ok {
			commands[nm] = pa
		} else if pa, ok := utils.LookPath(executableFolder, "netsnmp/"+nm); ok {
			commands[nm] = pa
		} else if pa, ok := utils.LookPath(executableFolder, "net-snmp/"+nm); ok {
			commands[nm] = pa
		}
	}

	if pa, ok := utils.LookPath(executableFolder, "tpt"); ok {
		commands["tpt"] = pa
	}
	if pa, ok := utils.LookPath(executableFolder, "nmap/nping"); ok {
		commands["nping"] = pa
	}
	if pa, ok := utils.LookPath(executableFolder, "nmap/nmap"); ok {
		commands["nmap"] = pa
	}
	if pa, ok := utils.LookPath(executableFolder, "putty/plink", "ssh"); ok {
		commands["plink"] = pa
		commands["ssh"] = pa
	}
	if pa, ok := utils.LookPath(executableFolder, "dig/dig", "dig"); ok {
		commands["dig"] = pa
	}
	if pa, ok := utils.LookPath(executableFolder, "ping"); ok {
		commands["ping"] = pa
	}
	if pa, ok := utils.LookPath(executableFolder, "tracert"); ok {
		commands["tracert"] = pa
	}
	if pa, ok := utils.LookPath(executableFolder, "traceroute"); ok {
		commands["traceroute"] = pa
	}
}

func Register(appRoot string, routeRegister func(string, http.Handler)) {
	if len(appRoot) == 0 {
		appRoot = `/`
	} else if !strings.HasSuffix(appRoot, `/`) {
		appRoot += `/`
	}
	routeRegister(appRoot+"replay", BuidHandler(Replay))
	routeRegister(appRoot+"ssh", BuidHandler(SSHShell))
	routeRegister(appRoot+"telnet", BuidHandler(TelnetShell))
	routeRegister(appRoot+"cmd", BuidHandler(ExecShell))
	routeRegister(appRoot+"cmd2", BuidHandler(ExecShell2))
	routeRegister(appRoot+"ssh_exec", BuidHandler(SSHExec))
}

func BuidHandler(handler func(*Context) error, middlewares ...func(*Context) error) websocket.Handler {
	return websocket.Handler(func(ws *websocket.Conn) {
		ctx := NewContext(ws)
		var err error
		for _, f := range middlewares {
			if err = f(ctx); err != nil {
				logString(ctx, err.Error())
				ws.Write([]byte(err.Error()))
				return
			}
		}
		err = handler(ctx)
		if err != nil {
			logString(ctx, err.Error())
			ws.Write([]byte(err.Error()))
		}
	})
}
