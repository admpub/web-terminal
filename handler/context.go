package handler

import (
	"strconv"
	"sync"

	"github.com/admpub/web-terminal/config"
	"golang.org/x/net/websocket"
)

var (
	SSHAccountContextKey         = struct{}{}
	SSHConfigContextKey          = struct{}{}
	SSHEndHostConfigContextKey   = struct{}{}
	SSHJumpHostConfigsContextKey = struct{}{}
)

type Context struct {
	*websocket.Conn
	Data   sync.Map
	Config *config.SSHConfig
}

func NewContext(ws *websocket.Conn) *Context {
	return &Context{
		Conn:   ws,
		Data:   sync.Map{},
		Config: &config.SSHConfig{},
	}
}

func (ctx *Context) GetSSHAccount() *config.AccountConfig {
	account, ok := ctx.Request().Context().Value(SSHAccountContextKey).(*config.AccountConfig)
	if ok {
		return account
	}
	user := ParamGet(ctx, "user")
	pwd := ParamGet(ctx, "password")
	charset := ParamGet(ctx, "charset")
	// Dial code is taken from the ssh package example
	account = &config.AccountConfig{
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

func (ctx *Context) GetHostConfig() (*config.HostConfig, error) {
	hostConfig, ok := ctx.Request().Context().Value(SSHAccountContextKey).(*config.HostConfig)
	if ok {
		return hostConfig, nil
	}
	hostname := ParamGet(ctx, "hostname")
	port := ParamGet(ctx, "port")
	if len(port) == 0 {
		port = "22"
	}
	portN, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	account := ctx.GetSSHAccount()
	hostConfig, err = config.NewHostConfigWithAccount(ctx.Conn, account, hostname, portN)
	if err != nil {
		return hostConfig, err
	}
	hostConfig.SetAccount(account)
	return hostConfig, err
}
