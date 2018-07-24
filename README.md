# web-terminal

A terminal in your browser using go.crypto/ssh and websocket. Based on [**xterm.js**](https://github.com/xtermjs/xterm.js).

# 打包静态资源
在当前项目根目录下执行：
```
go get github.com/GeertJohan/go.rice
go get github.com/GeertJohan/go.rice/rice
rice embed-go
```

# 访问地址
访问网址：http://127.0.0.1:37079/static/terminal.html?hostname=192.168.1.43  
该网址支持的参数：url_prefix，protocol，hostname，file，port，cmd，debug，user，password  
>其中，hostname为SSH服务器的主机名，port为SSH服务器的端口，user为SSH账号名，password为SSH密码