package goproxy

import (
	"context"
	"fmt"
	"github.com/yudeguang/17vinsoft/common"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type clsTaskExecuter struct {
	taskName   string
	moduleDir  string //模块的路径
	configFile string //动态配置文件路径
	exeFile    string //可执行程序路径
}

func (this *clsTaskExecuter) doTask(name string, rw http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	this.taskName = strings.TrimSpace(name)
	var err error = nil
	var hasWriteResulted = false
	//退出时根据是否有错误返回
	defer func() {
		if !hasWriteResulted {
			if err != nil {
				info := "ERR:" + err.Error()
				log.Println("返回错误:" + info)
				rw.Write([]byte(info))
			} else {
				rw.Write([]byte("ERR:没有返回任何信息和错误"))
			}
		}
	}()
	//准备任务的执行环节
	err = this.prepareTaskEnv()
	if err != nil {
		return
	}
	//生成动态执行参数
	var nTimeoutSecond int = 60
	var lstParams = []string{"##auto create by proxy"}
	for key, lst := range req.Form {
		var value = ""
		if len(lst) > 0 {
			value = lst[0]
		}
		if strings.ToLower(key) == "timeoutsecond" {
			n, err := strconv.Atoi(value)
			if err == nil && n > 0 {
				nTimeoutSecond = n
			}
		}
		lstParams = append(lstParams, fmt.Sprintf("%s=%s", key, value))
	}
	lstParams = append(lstParams, "proxy_data_dir="+common.ToAbsPath(saveProxyDataDir))
	iniText := strings.Join(lstParams, "\r\n")
	if err = ioutil.WriteFile(this.configFile, []byte(iniText), 644); err != nil {
		return
	}
	//删除结果文件后执行程序
	resultFile := filepath.Join(this.moduleDir, "result.dat")
	os.Remove(resultFile)
	err = this.executeProgram(this.exeFile, nTimeoutSecond)
	if err != nil {
		err = fmt.Errorf("执行程序错误:%v", err)
		return
	}
	//提取数据并返回
	file, err := os.Open(resultFile)
	defer file.Close()
	if err != nil {
		err = fmt.Errorf("没有生成结果文件:%v", resultFile)
		return
	}
	//下面自己返回信息
	hasWriteResulted = true
	//读取前面2个字节,如果是PK就要将头设置为zip压缩文件
	data := make([]byte, 2)
	file.Read(data)
	file.Seek(0, io.SeekStart)
	if data[0] == 'P' && data[1] == 'K' {
		file.Close()
		log.Println("返回结果是压缩文件:")
		http.ServeFile(rw, req, resultFile)
		return
	} else {
		log.Println("返回结果是普通文件")
		io.Copy(rw, file)
	}
	file.Close()
}
func (this *clsTaskExecuter) prepareTaskEnv() error {
	if this.taskName == "" {
		return fmt.Errorf("task任务为空")
	}
	this.moduleDir = filepath.Join("./module", this.taskName)
	this.configFile = filepath.Join(this.moduleDir, "dynamic_config.ini")
	this.exeFile = filepath.Join(common.GetExePath(), "module", this.taskName, this.taskName+".exe")
	var err error = nil
	if _, err = os.Stat(this.moduleDir); err != nil {
		return err
	}
	if _, err = os.Stat(this.exeFile); err != nil {
		return err
	}
	return nil
}

func (this *clsTaskExecuter) executeProgram(exeFile string, timeoutSecond int) error {
	log.Println("执行程序:", exeFile)
	log.Println("等待超时:", timeoutSecond, "秒")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(timeoutSecond))
	defer cancel()
	cmd := exec.CommandContext(ctx, exeFile)
	cmd.Dir = filepath.Dir(exeFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	var err = cmd.Start()
	if err != nil {
		return err
	}
	cmd.Wait()
	err = nil
	select {
	case <-ctx.Done():
		err = fmt.Errorf("执行超时:%v s", timeoutSecond)
		log.Println(err)
		break
	default:
		log.Println("执行程序完成 ")
		break
	}
	return err
}
