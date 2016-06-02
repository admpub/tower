# Tower

Tower 是一个为golang的web开发者提供的工具。它会动态监控文件更改并自动重新编译运行您的golang源码。
它采用了反向代理的方式，自动将用户的访问代理到新的程序，然后关闭并删除旧程序，这样就可以最大限度的做到零下线升级您的golang应用。
如果编译失败或出现异常，Tower会通过一个整洁的页面显示这些信息。

## 安装
```bash
go get github.com/admpub/tower
```

## 使用方法

```bash
cd your/project
tower # 现在访问 localhost:8080
```

Tower 在默认情况下假设你golang应用的main文件为 _main.go_，端口为 _5000-5050_。你可以按如下方式更改它:

```bash
tower -m app.go -p 3000-4000
```

或把它们放入配置文件:

```bash
tower init
vim .tower.yml
tower
```

## 常见问题

#### 'Too many open files'

运行下面的命令提高进程可打开的文件数量:

```bash
ulimit -S -n 2048 # OSX
```

## 工作原理

```
浏览器访问: http://localhost:8080
      \/
tower (监听 8080 端口)
      \/ (反向代理)
你的golang应用 (监听 5000 至 5050 中的任意一个端口)
```

所有来自localhost:8080的提交Tower都会转发给你的应用。
转发使用的是 _[httputil.ReverseProxy](http://golang.org/pkg/net/http/httputil/#ReverseProxy)_。
在转发之前，如果您的应用没有运行或文件被更改，Tower将在其它进程中自动编译并运行你的应用; 
Tower 使用了 _[howeyc/fsnotify](https://github.com/howeyc/fsnotify)_ 来监控文件更改。

## 管理接口
通过管理接口您可以临时关闭自动编译功能。

      默认情况下，只有本地可以访问管理接口，您可以通过在配置文件中设置`admin_pwd`(指定访问密码，通过在网址中增加“?pwd=<你的密码>”来访问)或`admin_ip`(指定允许访问的IP地址，多个用半角逗号隔开)来灵活设置。

要临时关闭自动编译功能只需要访问：http://localhost:8080/tower-proxy/watch/pause

重新开启自动编译：http://localhost:8080/tower-proxy/watch/begin

查看是否开启自动编译：http://localhost:8080/tower-proxy/watch

## License

Tower is released under the [MIT License](http://www.opensource.org/licenses/MIT).
