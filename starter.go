package goproxy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/yudeguang/17vinsoft/common"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

//需要放弃的连接
var lstAbortSites = []string{".google.com", ".googleapis.com", ".googleusercontent.com"}

//需要忽略的文件后缀
var lstFilterExtName = ".jpg.gif.css.js.png.jpeg.ico"

//执行任务请求地址
var uriTaskPrefix = "/proxy_17vin_task/"

//忽略这个数据
var ErrIgnoreReponse = errors.New("ignore request not useragent info")

//一个请求的分割符
var lineSplit = "\r\n------HTTP_PROXY_SPLIT------\r\n"

//启动函数
func StartProxy() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	err := pConfiger.LoadConfig()
	if err != nil {
		log.Println("读取配置文件失败:", err)
		os.Exit(-1)
	}
	if _, err = os.Stat(saveProxyDataDir); err != nil {
		os.Mkdir(saveProxyDataDir, 644)
		time.Sleep(100 * time.Millisecond)
		if _, err = os.Stat(saveProxyDataDir); err != nil {
			log.Println("无法创建数据保存目录:" + saveProxyDataDir)
			os.Exit(-1)
		}
	}
	proxy := New(WithDelegate(&MyRequestDelegate{}), WithDecryptHTTPS(&MyCertCache{}))
	server := &http.Server{
		Addr:         pConfiger.ListenAddr,
		Handler:      proxy,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	log.Println("启动地址 ", pConfiger.ListenAddr, " 上的HTTP代理服务...")
	log.Println("可通过", fmt.Sprintf("http://127.0.0.1:9006%staskname?var=val", uriTaskPrefix), "验证执行任务")
	go startHttpTaskServer()
	go removeDataThead()
	go memoryCheckThread()
	go startSeleniumServer()
	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

// Cache 证书缓存接口
var mapCache = make(map[string]*tls.Certificate)

type MyCertCache struct {
}

func (*MyCertCache) Set(host string, c *tls.Certificate) {
	mapCache[host] = c
}
func (*MyCertCache) Get(host string) *tls.Certificate {
	return mapCache[host]
}

//启动任务请求http
func startHttpTaskServer() {
	http.Handle("/", &TaskHttpHandler{})
	var err = http.ListenAndServe(":9006", nil)
	panic("启动错误:" + err.Error())
}

//移除生成的数据
func removeDataThead() {
	for {
		lstFs, err := ioutil.ReadDir(saveProxyDataDir)
		if err == nil {
			for _, fs := range lstFs {
				if fs.IsDir() {
					dir := filepath.Join(saveProxyDataDir, fs.Name())
					err := removeFileByDirectory(dir)
					if err != nil && err.Error() == "nofiles" {
						os.RemoveAll(dir)
					}
				}
			}
		}
		time.Sleep(time.Second * 300)
	}
}

var regLogFilePattern = regexp.MustCompile("^(\\d{14}.)")

func removeFileByDirectory(dir string) error {
	lstFs, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(lstFs) == 0 {
		return fmt.Errorf("nofiles")
	}
	nowTime := time.Now()
	maxFile := nowTime.Add(time.Minute * 5).Format("20060102150405") //比当前时间多5分钟之后的都删掉
	minFile := nowTime.Add(time.Minute * time.Duration(0-pConfiger.FileSaveMinute)).Format("20060102150405")
	//log.Println("minTime:",minFile)
	//log.Println("maxTime:",maxFile)
	for _, fs := range lstFs {
		if fs.IsDir() {
			continue
		}
		var name = fs.Name()
		if !regLogFilePattern.MatchString(name) {
			continue
		}
		if name < minFile || name > maxFile {
			//删除这个文件
			dst := filepath.Join(dir, name)
			os.Remove(dst)
		}
	}
	return nil
}

//内存使用检测Thread
func memoryCheckThread() {
	for {
		time.Sleep(time.Second * 60)
		runtime.GC()
		debug.FreeOSMemory()
	}
}

//启动java,selenium-server-standalone-3.141.59.jar
func startSeleniumServer() {
	binPath := filepath.Join(common.GetExePath(), "bins")
	var chromeDriver = filepath.Join(binPath, "chromedriver.exe")
	var seleniumJar = filepath.Join(binPath, "selenium-server-standalone.jar")
	if _, err := os.Stat(seleniumJar); err != nil {
		//不存在这个jar，不用运行
		return
	}
	log.Println("发现WebDriver Sever:" + seleniumJar)
	//因为执行jar文件必须要java程序
	_, err := exec.LookPath("java.exe")
	if err != nil {
		log.Println("ERROR:没有发现java环境:" + err.Error())
		return
	}
	//这里结束一下所有的chrome.exe和chromedriver.exe进程，怕上次的进程还保留着
	exec.Command("taskkill.exe", "/F", "/IM", "chromedriver.exe", "/T").Run()
	exec.Command("taskkill.exe", "/F", "/IM", "chrome.exe", "/T").Run()
	for {
		var args = []string{}
		args = append(args, "-Dwebdriver.chrome.driver="+chromeDriver)
		args = append(args, "-jar", seleniumJar)
		//-timeout这个参数是多少秒没有操作就关闭一个会话(selenium.WebDriver这种对象，也就是一个浏览器)
		//主要是节省资源和防止本程序异常退出了,5分钟差不多，相当于5分钟没有操作就退出浏览器
		//下次操作再启动并登录一下
		args = append(args, "-timeout", fmt.Sprint(pConfiger.SessionTimeout))
		//-browserTimeout好像是说页面没有响应的时间，最小设置是60秒
		args = append(args, "-browserTimeout", "60")
		log.Println("执行程序:java.exe " + strings.Join(args, " "))
		cmd := exec.Command("java.exe", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Println("执行程序错误:", err)
		}
		time.Sleep(5 * time.Second)
	}
}
