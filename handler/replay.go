package handler

import (
	"io"
	"os"
	"runtime"

	"golang.org/x/net/websocket"
)

// Replay .
func Replay(ws *websocket.Conn) {
	defer ws.Close()
	fileName := ParamGet(ws, "file")
	charset := ParamGet(ws, "charset")
	if 0 == len(charset) {
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
