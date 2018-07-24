package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	rice "github.com/GeertJohan/go.rice"
	"github.com/admpub/web-terminal/config"
	"github.com/admpub/web-terminal/handler"
	"golang.org/x/net/websocket"
)

func init() {
	config.FlagParse()
}

func main() {
	if nil != flag.Args() && 0 != len(flag.Args()) {
		flag.Usage()
		return
	}
	config.Default.SetDefault()

	appRoot := config.Default.APPRoot
	http.Handle(appRoot+"replay", websocket.Handler(handler.Replay))
	http.Handle(appRoot+"ssh", websocket.Handler(handler.SSHShell))
	http.Handle(appRoot+"telnet", websocket.Handler(handler.TelnetShell))
	http.Handle(appRoot+"cmd", websocket.Handler(handler.ExecShell))
	http.Handle(appRoot+"cmd2", websocket.Handler(handler.ExecShell2))
	http.Handle(appRoot+"ssh_exec", websocket.Handler(handler.SSHExec))

	templateBox, err := rice.FindBox("static")
	if err != nil {
		fmt.Println(errors.New("load static directory fail, " + err.Error()))
		return
	}
	httpFS := http.FileServer(templateBox.HTTPBox())

	http.Handle("/static/", http.StripPrefix("/static/", httpFS))
	if appRoot != "/" {
		http.Handle(appRoot+"static/", http.StripPrefix(appRoot+"static/", httpFS))
	}
	fmt.Println("[web-terminal] listen at '" + config.Default.Listen + "' with root is '" + config.Default.ResourceDir + "'")
	err = http.ListenAndServe(config.Default.Listen, nil)
	if err != nil {
		fmt.Println("ListenAndServe: " + err.Error())
	}
}
