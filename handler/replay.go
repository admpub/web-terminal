package handler

import (
	"io"
	"os"
	"runtime"

	"golang.org/x/net/websocket"
)

func Replay(ws *websocket.Conn) {
	defer ws.Close()
	fileName := ws.Request().URL.Query().Get("file")
	charset := ws.Request().URL.Query().Get("charset")
	if "" == charset {
		if "windows" == runtime.GOOS {
			charset = "GB18030"
		} else {
			charset = "UTF-8"
		}
	}
	dumpOut, err := os.Open(fileName)
	if nil != err {
		logString(ws, "open '"+fileName+"' failed:"+err.Error())
		return
	}
	defer dumpOut.Close()

	if _, err := io.Copy(decodeBy(charset, ws), dumpOut); err != nil {
		logString(ws, "copy of stdout failed:"+err.Error())
		return
	}
}
