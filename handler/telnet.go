package handler

import (
	"io"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/admpub/web-terminal/config"
	"github.com/admpub/web-terminal/library/telnet"
	"golang.org/x/net/websocket"
)

func TelnetShell(ws *websocket.Conn) {
	defer ws.Close()
	hostname := ws.Request().URL.Query().Get("hostname")
	port := ws.Request().URL.Query().Get("port")
	if "" == port {
		port = "23"
	}
	charset := ws.Request().URL.Query().Get("charset")
	if "" == charset {
		if "windows" == runtime.GOOS {
			charset = "GB18030"
		} else {
			charset = "UTF-8"
		}
	}
	//columns := toInt(ws.Request().URL.Query().Get("columns"), 80)
	//rows := toInt(ws.Request().URL.Query().Get("rows"), 40)

	var dumpOut io.WriteCloser
	var dumpIn io.WriteCloser

	client, err := net.Dial("tcp", hostname+":"+port)
	if nil != err {
		logString(ws, "Failed to dial: "+err.Error())
		return
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
	if "true" == strings.ToLower(ws.Request().URL.Query().Get("debug")) {
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
		logString(nil, "failed to create connection: "+e.Error())
		return
	}
	columns := toInt(ws.Request().URL.Query().Get("columns"), 80)
	rows := toInt(ws.Request().URL.Query().Get("rows"), 40)
	conn.SetWindowSize(byte(rows), byte(columns))

	go func() {
		_, err := io.Copy(decodeBy(charset, client), warp(ws, dumpOut))
		if nil != err {
			logString(nil, "copy of stdin failed:"+err.Error())
		}
	}()

	if _, err := io.Copy(decodeBy(charset, ws), conn); err != nil {
		logString(ws, "copy of stdout failed:"+err.Error())
		return
	}
}
