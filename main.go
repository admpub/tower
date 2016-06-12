package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/admpub/confl"
)

const ConfigName = ".tower.yml"

var (
	_appMainFile   *string
	_appPort       *string
	_pxyPort       *string
	_appBuildDir   *string
	_portParamName *string
	_runParams     *string
	_verbose       *bool
	_configFile    *string
	_adminPwd      *string
	_adminIPs      *string

	app   App
	build string = "1"
)

func main() {
	_appMainFile = flag.String("m", "", "path to your app's main file.")
	_appPort = flag.String("p", "5000-5050", "port range of your app.")
	_pxyPort = flag.String("r", "8080", "proxy port of your app.")
	_appBuildDir = flag.String("o", "", "save the executable file the folder.")
	_portParamName = flag.String("n", "", "app's port param name.")
	_runParams = flag.String("s", "", "app's run params.")
	_verbose = flag.Bool("v", false, "show more stuff.")
	_configFile = flag.String("c", ConfigName, "yaml configuration file location.")
	_adminPwd = flag.String("w", "", "admin password.")
	_adminIPs = flag.String("i", "127.0.0.1,::1", "admin allow IP.")

	flag.Parse()

	args := flag.Args()
	if len(args) == 1 && args[0] == "init" {
		generateExampleConfig()
		return
	}
	startTower()
}

func generateExampleConfig() {
	_, file, _, _ := runtime.Caller(0)
	exampleConfig := filepath.Join(filepath.Dir(file), "tower.yml")
	exec.Command("cp", exampleConfig, ConfigName).Run()
	fmt.Println("== Generated config file " + ConfigName)
}

func atob(a string) bool {
	return a == `1` || a == `true` || a == `on` || a == `yes`
}

func startTower() {
	var (
		appMainFile        = *_appMainFile
		appPort            = *_appPort
		pxyPort            = *_pxyPort
		appBuildDir        = *_appBuildDir
		portParamName      = *_portParamName
		runParams          = *_runParams
		configFile         = *_configFile
		verbose            = *_verbose
		adminPwd           = *_adminPwd
		adminIPs           = *_adminIPs
		allowBuild         = build == "1"
		suffix             = ".exe"
		_suffix            = ""
		watchedFiles       string
		watchedOtherDir    string
		ignoredPathPattern string
		offlineMode        bool
		disabledLogRequest bool
	)
	if configFile == "" {
		configFile = ConfigName
	}
	contents, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Println(err)
	} else {
		newmap := map[string]string{}
		yamlErr := confl.Unmarshal(contents, &newmap)
		if yamlErr != nil {
			fmt.Println(yamlErr)
		}
		appPort, _ = newmap["app_port"]
		pxyPort, _ = newmap["pxy_port"]
		appBuildDir, _ = newmap["app_buildDir"] //编译模式下有效
		portParamName, _ = newmap["app_portParamName"]
		runParams, _ = newmap["app_runParams"]
		watchedFiles, _ = newmap["watch"]
		watchedOtherDir, _ = newmap["watch_otherDir"] //编译模式下有效
		ignoredPathPattern, _ = newmap["watch_ignoredPath"]
		offlineModeStr, _ := newmap["offline_mode"]

		if v, ok := newmap["admin_pwd"]; ok {
			adminPwd = v
		}
		if v, ok := newmap["admin_ip"]; ok {
			adminIPs = v
		}
		if atob(offlineModeStr) {
			offlineMode = true
		}
		if logRequestStr, ok := newmap["log_request"]; ok {
			disabledLogRequest = atob(logRequestStr) == false
		}
		if pxyPort == "" {
			pxyPort = ProxyPort
		}
		if allowBuild {
			appMainFile, _ = newmap["main"] //编译模式下有效
		} else {
			appMainFile, _ = newmap["exec"] //非编译模式下有效
			if appMainFile == "" {
				fmt.Println("请设置exec参数用来指定执行文件位置")
				time.Sleep(time.Second * 10)
				return
			}
			f, err := os.Open(appMainFile)
			if err == nil {
				_, err = f.Stat()
			}
			f.Close()
			if err != nil {
				fmt.Println(err)
				time.Sleep(time.Second * 10)
				return
			}
			fileName := filepath.Base(appMainFile)
			AppBin = fileName
			if strings.HasSuffix(AppBin, suffix) {
				AppBin = strings.TrimSuffix(AppBin, suffix)
				_suffix = suffix
			}
			nameOk := strings.HasPrefix(AppBin, BinPrefix)
			if nameOk {
				fileName := strings.TrimPrefix(AppBin, BinPrefix)
				_, err = strconv.ParseInt(fileName, 10, 64)
				if err != nil {
					nameOk = false
				}
			}
			if !nameOk {
				fmt.Println("exec参数指定的可执行文件名称格式应该为：", BinPrefix+"0"+_suffix, "。")
				fmt.Println("其中的“0”是代表版本号的整数，请修改为此格式。")
				time.Sleep(time.Second * 300)
				return
			}
		}
	}

	err = dialAddress("127.0.0.1:"+appPort, 1)
	if err == nil {
		fmt.Println("Error: port (" + appPort + ") already in used.")
		os.Exit(1)
	}

	if verbose {
		fmt.Println("== Application Info")
		fmt.Printf("  build app with: %s\n", appMainFile)
		fmt.Printf("  redirect requests from localhost:%s to localhost:%s\n\n", ProxyPort, appPort)
	}

	app = NewApp(appMainFile, appPort, appBuildDir, portParamName)
	app.OfflineMode = offlineMode
	app.DisabledLogRequest = disabledLogRequest
	if runParams != `` {
		app.RunParams = strings.Split(runParams, ` `)
	}
	if watchedOtherDir != "" {
		watchedOtherDir += "|" + app.Root
	}
	watcher := NewWatcher(watchedOtherDir, watchedFiles, ignoredPathPattern)
	proxy := NewProxy(&app, &watcher)
	proxy.AdminPwd = adminPwd
	if adminIPs != `` {
		proxy.AdminIPs = strings.Split(adminIPs, `,`)
	}
	if allowBuild {
		watcher.OnChanged = func(file string) {
			fmt.Println(`== Build Mode.`)
			watcher.Reset()
			fileName := filepath.Base(file)
			if strings.HasPrefix(fileName, BinPrefix) {
				fmt.Println(`忽略`, fileName, `更改`)
				return
			}
			if !app.SupportMutiPort() {
				fmt.Println(`Unspecified switchable other ports.`)
				return
			}
			port := app.UseRandPort()
			for i := 0; i < 3 && port == app.Port; i++ {
				app.Clean()
				time.Sleep(time.Second)
				port = app.UseRandPort()
			}
			if port == app.Port {
				fmt.Println(`取得的端口与当前端口相同，无法编译切换`)
				return
			}
			err = app.Start(true, port)
			if err != nil {
				fmt.Println(err)
			}
		}
	} else {
		watcher.OnChanged = func(file string) {
			fmt.Println(`== Switch Mode.`)
			watcher.Reset()
			if !app.SupportMutiPort() {
				fmt.Println(`Unspecified switchable other ports.`)
				return
			}
			port := app.UseRandPort()
			for i := 0; i < 3 && port == app.Port; i++ {
				app.Clean()
				time.Sleep(time.Second)
				port = app.UseRandPort()
			}
			if port == app.Port {
				fmt.Println(`取得的端口与当前端口相同，无法切换`)
				return
			}

			fileName := filepath.Base(file)
			if !strings.HasPrefix(fileName, BinPrefix) {
				fmt.Println(`忽略非`, BinPrefix, `前缀文件更改`)
				return
			}
			if _suffix != "" {
				fileName = strings.TrimSuffix(fileName, _suffix)
			}
			newAppBin := fileName
			fileName = strings.TrimPrefix(fileName, BinPrefix)
			newFileTs, err := strconv.ParseInt(fileName, 10, 64)
			if err != nil {
				fmt.Println(err)
				return
			}
			fileName = strings.TrimPrefix(AppBin, BinPrefix)
			oldFileTs, err := strconv.ParseInt(fileName, 10, 64)
			if err != nil {
				fmt.Println(err)
				return
			}
			if newFileTs <= oldFileTs {
				fmt.Println(`新文件时间戳小于旧文件，忽略`)
				return
			}
			AppBin = newAppBin
			err = app.Start(true, port)
			if err != nil {
				fmt.Println(err)
			}
		}
		watcher.OnlyWatchBin = true
		app.DisabledBuild = true
	}
	proxy.Port = pxyPort
	go func() {
		mustSuccess(watcher.Watch())
	}()
	err = app.Start(true, app.Port)
	if err != nil {
		fmt.Println(err)
	}
	mustSuccess(proxy.Listen())
}
