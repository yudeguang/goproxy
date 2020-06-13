package goproxy

import (
	"encoding/xml"
	"github.com/yudeguang/17vinsoft/common"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
)

var saveProxyDataDir = "./temp_proxy_data"
var pConfiger = &clsConfiger{}

type clsConfiger struct {
	XMLName        xml.Name       `xml:"Config"`
	DataSavePath   string         //抓取到的文件存储目录
	FileSaveMinute int            //文件最多保留多少分钟之前的文件
	ListenAddr     string         //listen的ip端口地址
	SessionTimeout int            //浏览器session无操作时退出时间，秒,默认600
	ValidHost      []string       `xml:"ValidHost>Host"` //需要记录的host,默认情况下所有都记录下来
	mapValidHost   map[string]int //ValidHost的map表示,加快查找
	localIPAddr    string         //当前IP地址用于判断循环代理
}

func (this *clsConfiger) LoadConfig() error {
	this.mapValidHost = make(map[string]int)
	var xmlFile = "./" + common.GetExeBaseName() + ".xml"
	var isFileExist = true
	if _, err := os.Stat(xmlFile); err != nil {
		//文件不存在,创建一个
		isFileExist = false
	} else {
		isFileExist = true
		data, err := ioutil.ReadFile(xmlFile)
		if err != nil {
			return err
		}
		if err = xml.Unmarshal(data, this); err != nil {
			return err
		}
	}
	//调整参数
	if this.DataSavePath != "" {
		saveProxyDataDir = filepath.Join(this.DataSavePath, saveProxyDataDir)
	}
	if this.FileSaveMinute < 5 {
		this.FileSaveMinute = 60
	}
	if this.SessionTimeout < 30 {
		this.SessionTimeout = 1200
	}
	if this.ListenAddr == "" {
		this.ListenAddr = "127.0.0.1:9005"
	}
	if !isFileExist {
		data, err := xml.MarshalIndent(this, "", "\t")
		if err == nil {
			ioutil.WriteFile(xmlFile, data, 644)
		}
	}
	for _, host := range this.ValidHost {
		this.mapValidHost[strings.ToLower(host)] = 1
	}
	//获得自己的ip地址存起来
	lstIPAddr := []string{}
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok {
				lstIPAddr = append(lstIPAddr, ipNet.IP.String())
			}
		}
	}
	this.localIPAddr = "#" + strings.ToLower(strings.Join(lstIPAddr, "#")) + "#"
	return nil
}
func (this *clsConfiger) IsValidHost(host string) bool {
	if host == "" || len(this.mapValidHost) == 0 {
		return true
	}
	host = strings.ToLower(host)
	v, ok := this.mapValidHost[host]
	if ok && v == 1 {
		return true
	}
	return false
}

func (this *clsConfiger) CheckIsSelfIP(dstHost string) bool {
	if npos := strings.Index(dstHost, ":"); npos > 0 {
		dstHost = dstHost[:npos]
	}
	dstHost = strings.ToLower(dstHost)
	if dstHost == "" {
		return false
	}
	if dstHost == "127.0.0.1" || dstHost == "localhost" {
		return true
	}
	if strings.Contains(this.localIPAddr, "#"+dstHost+"#") {
		return true
	}
	return false
}
