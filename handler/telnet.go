package handler

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/admpub/web-terminal/config"
	"github.com/admpub/web-terminal/library/telnet"
)

func TelnetShell(ctx *Context) error {
	defer ctx.Close()
	hostname := ParamGet(ctx, "hostname")
	port := ParamGet(ctx, "port")
	if 0 == len(port) {
		port = "23"
	}
	charset := fixCharset(ParamGet(ctx, "charset"))
	//columns := toInt(ParamGet(ctx,"columns"), 80)
	//rows := toInt(ParamGet(ctx,"rows"), 40)

	var dumpOut io.WriteCloser
	var dumpIn io.WriteCloser
	ws := ctx.Conn
	client, err := net.Dial("tcp", hostname+":"+port)
	if nil != err {
		return fmt.Errorf("Failed to dial: %w", err)
	}
	defer func() {
		client.Close()
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

	if debug {
		var err error
		dumpOut, err = os.OpenFile(config.Default.LogDir+hostname+".dump_telnet_out.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if nil != err {
			dumpOut = nil
		}
		dumpIn, err = os.OpenFile(config.Default.LogDir+hostname+".dump_telnet_in.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		if nil != err {
			dumpIn = nil
		}
	}

	conn, e := telnet.NewConnWithRead(client, warp(client, dumpIn))
	if nil != e {
		return fmt.Errorf("Failed to create connection: %w", err)
	}
	columns := toInt(ParamGet(ctx, "columns"), 80)
	rows := toInt(ParamGet(ctx, "rows"), 40)
	conn.SetWindowSize(byte(rows), byte(columns))

	go func() {
		_, err := io.Copy(decodeBy(charset, client), warp(ws, dumpOut))
		if nil != err {
			logString(nil, "copy of stdin failed:"+err.Error())
		}
	}()

	if _, err := io.Copy(decodeBy(charset, ws), conn); err != nil {
		return fmt.Errorf("copy of stdout failed: %w", err)
	}
	return err
}
