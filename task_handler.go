package goproxy

import (
	"github.com/yudeguang/file"
	"io"
	"log"
	"net/http"
	"strings"
)

type TaskHttpHandler struct {
}

func (p *TaskHttpHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	if strings.HasPrefix(req.URL.Path, uriTaskPrefix) {
		taskName := req.URL.Path[len(uriTaskPrefix):]
		log.Println("请求执行任务:", taskName)
		executer := &clsTaskExecuter{}
		executer.doTask(taskName, rw, req)
		return
	}
	http.NotFound(rw, req)
}

//获取当前地址
func programAdress(rw http.ResponseWriter, req *http.Request) {
	io.WriteString(rw, file.CurrentDir())
}
