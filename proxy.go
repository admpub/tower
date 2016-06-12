package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const ProxyPort = "8080"

type Proxy struct {
	App          *App
	appOldPort   string
	ReserveProxy *httputil.ReverseProxy
	Watcher      *Watcher
	FirstRequest *sync.Once
	upgraded     int64
	Port         string
	AdminPwd     string
	AdminIPs     []string
}

func NewProxy(app *App, watcher *Watcher) (proxy Proxy) {
	proxy.App = app
	proxy.Watcher = watcher
	proxy.Port = ProxyPort
	proxy.AdminIPs = []string{`127.0.0.1`, `::1`}
	return
}

func (this *Proxy) authAdmin(r *http.Request) bool {
	query := r.URL.Query()
	pwd := query.Get(`pwd`)
	valid := false
	if pwd != `` || pwd == this.AdminPwd {
		valid = true
	} else {
		clientIP := r.RemoteAddr
		if p := strings.LastIndex(clientIP, `]:`); p > -1 {
			clientIP = clientIP[0:p]
			clientIP = strings.TrimPrefix(clientIP, `[`)
		} else if p := strings.LastIndex(clientIP, `:`); p > -1 {
			clientIP = clientIP[0:p]
		}
		for _, ip := range this.AdminIPs {
			if ip == clientIP {
				valid = true
				break
			}
		}
	}
	return valid
}

func (this *Proxy) SetBody(code int, body []byte, w http.ResponseWriter) {
	if code == 0 {
		code = http.StatusOK
	}
	w.WriteHeader(code)
	w.Write(body)
}

func (this *Proxy) Listen() (err error) {
	fmt.Println("== Listening to http://localhost:" + this.Port)
	this.SetReserveProxy()
	this.FirstRequest = &sync.Once{}

	http.HandleFunc("/tower-proxy/watch/pause", func(w http.ResponseWriter, r *http.Request) {
		status := `done`
		if !this.authAdmin(r) {
			status = `Authentication failed`
		} else {
			this.Watcher.Paused = true
		}
		this.SetBody(0, []byte(status), w)
	})

	http.HandleFunc("/tower-proxy/watch/begin", func(w http.ResponseWriter, r *http.Request) {
		status := `done`
		if !this.authAdmin(r) {
			status = `Authentication failed`
		} else {
			this.Watcher.Paused = false
		}
		this.SetBody(0, []byte(status), w)
	})

	http.HandleFunc("/tower-proxy/watch", func(w http.ResponseWriter, r *http.Request) {
		status := `OK`
		if this.Watcher.Paused {
			status = `Pause`
		}
		this.SetBody(0, []byte(`watch status: `+status), w)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		this.ServeRequest(w, r)
	})
	return http.ListenAndServe(":"+this.Port, nil)
}

func (this *Proxy) SetReserveProxy() {
	fmt.Println("== Proxy to http://localhost:" + this.App.Port)
	this.appOldPort = this.App.Port
	url, _ := url.ParseRequestURI("http://localhost:" + this.App.Port)
	this.ReserveProxy = httputil.NewSingleHostReverseProxy(url)
}

func (this *Proxy) ServeRequest(w http.ResponseWriter, r *http.Request) {
	mw := ResponseWriterWrapper{ResponseWriter: w}
	if !this.App.DisabledLogRequest {
		this.logStartRequest(r)
		defer this.logEndRequest(&mw, r, time.Now())
	}

	if this.App.SwitchToNewPort {
		fmt.Println(`== Switch port:`, this.appOldPort, `=>`, this.App.Port)
		this.App.SwitchToNewPort = false
		this.SetReserveProxy()
		this.FirstRequest.Do(func() {
			this.ReserveProxy.ServeHTTP(&mw, r)
			this.upgraded = time.Now().Unix()
			go this.App.Clean()
			this.FirstRequest = &sync.Once{}
		})
	} else if !this.App.IsRunning() || this.Watcher.Changed {
		this.Watcher.Reset()
		err := this.App.Restart()
		if err != nil {
			RenderBuildError(&mw, this.App, err.Error())
			return
		}

		this.FirstRequest.Do(func() {
			this.ReserveProxy.ServeHTTP(&mw, r)
			this.FirstRequest = &sync.Once{}
		})
	}

	this.App.LastError = ""
	if this.upgraded > 0 {
		timeout := time.Now().Unix() - this.upgraded
		if timeout > 3600 {
			this.upgraded = 0
		}
		mw.Header().Set(`X-Server-Upgraded`, fmt.Sprintf("%v", timeout))
	}

	if !mw.Processed {
		this.ReserveProxy.ServeHTTP(&mw, r)
	}

	if len(this.App.LastError) != 0 {
		RenderAppError(&mw, this.App, this.App.LastError)
	} else if this.App.IsQuit() {
		fmt.Println("== App quit unexpetedly")
		this.App.Start(false)
		RenderError(&mw, this.App, "App quit unexpetedly.")
	}
}

var staticExp = regexp.MustCompile(`\.(png|jpg|jpeg|gif|svg|ico|swf|js|css|html|woff)`)

func (this *Proxy) isStaticRequest(uri string) bool {
	return staticExp.Match([]byte(uri))
}

func (this *Proxy) logStartRequest(r *http.Request) {
	if !this.isStaticRequest(r.RequestURI) {
		fmt.Printf("\n\n\nStarted %s \"%s\" at %s\n", r.Method, r.RequestURI, time.Now().Format("2006-01-02 15:04:05 +700"))
		params := this.formatRequestParams(r)
		if len(params) > 0 {
			fmt.Printf("  Parameters: %s\n", params)
		}
	}
}

type MyReadCloser struct {
	bytes.Buffer
}

func (this *MyReadCloser) Close() error {
	return nil
}

func (this *Proxy) formatRequestParams(r *http.Request) string {
	// Keep an copy of request Body, and restore it after parsed form.
	var b1, b2 MyReadCloser
	io.Copy(&b1, r.Body)
	io.Copy(&b2, &b1)
	r.Body = &b1
	r.ParseForm()
	r.Body = &b2

	if r.Form == nil {
		return ""
	}

	var params []string
	for key, vals := range r.Form {
		var strVals []string
		for _, val := range vals {
			strVals = append(strVals, `"`+val+`"`)
		}
		params = append(params, `"`+key+`":[`+strings.Join(strVals, ", ")+`]`)
	}
	return strings.Join(params, ", ")
}

func (this *Proxy) logEndRequest(mw *ResponseWriterWrapper, r *http.Request, startTime time.Time) {
	if !this.isStaticRequest(r.RequestURI) {
		fmt.Printf("Completed %d in %dms\n", mw.Status, time.Since(startTime)/time.Millisecond)
	}
}

// A response Wrapper to capture request's status code.
type ResponseWriterWrapper struct {
	Status    int
	Processed bool
	http.ResponseWriter
}

func (this *ResponseWriterWrapper) WriteHeader(status int) {
	this.Status = status
	this.Processed = true
	this.ResponseWriter.WriteHeader(status)
}
