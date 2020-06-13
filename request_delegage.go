package goproxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//数据处理接
type MyRequestDelegate struct {
}

func (this *MyRequestDelegate) Connect(ctx *Context, rw http.ResponseWriter) {
	var requri = ctx.Req.RequestURI
	for _, v := range lstAbortSites {
		if strings.Contains(requri, v) {
			ctx.Abort()
			return
		}
	}
	//log.Println("请求地址:", ctx.Req.RequestURI)
}

func (this *MyRequestDelegate) Auth(ctx *Context, rw http.ResponseWriter) {

}

func (this *MyRequestDelegate) BeforeRequest(ctx *Context) {
	if ctx.Req == nil{
		return;
	}
	log.Println("请求连接:",ctx.Req.URL.String())
	if ctx.Req.Method != "POST"{
		return;
	}
	if ctx.Req.ContentLength>1024*1024{
		//大于1M也算了，应该没有这么多
		return;
	}

	newBuffer := bytes.NewBuffer(nil);
	io.Copy(newBuffer,ctx.Req.Body);
	ctx.Req.Body = ioutil.NopCloser(newBuffer);
	ctx.textBody = newBuffer.String();
}

func (this *MyRequestDelegate) BeforeResponse(ctx *Context, resp *http.Response, err error) {
	if err != nil || ctx==nil || ctx.Req == nil || resp == nil{ //请求有错误的,直接丢弃
		return;
	}
	//以host为目录存储数据
	hostName := ctx.Req.Host
	if npos := strings.Index(hostName,":");npos>0{
		hostName = hostName[:npos]
	}
	if hostName == ""{
		return;
	}
	//检查host
	if !pConfiger.IsValidHost(hostName){//不用记录的host
		return;
	}
	//检查返回数据类型
	if !this.checkIsValidByContentType(resp.Header.Get("Content-Type")){
		return;
	}
	requestURI := ctx.Req.RequestURI
	//log.Println("请求:\r\nUserAgent:",userAgent,"\r\nRequestURI:"+requestURI)
	//检查请求扩展名,有些特殊后缀的不需要
	_,extName := this.fetchRequestFileAndExt(requestURI);
	if(extName == "" || strings.Index(lstFilterExtName,extName) == -1){//处理文件保存
		this.saveResponseData(hostName,ctx,resp);
	}
}

func (this *MyRequestDelegate) ParentProxy(req *http.Request) (*url.URL, error) {
	//return http.ProxyFromEnvironment(req)
	return nil, nil
}

func (this *MyRequestDelegate) Finish(ctx *Context) {

}

func (this *MyRequestDelegate) ErrorLog(err error) {
	//log.Println("NONONO:", err)
}
//分离出来请求文件名和后缀
func (this* MyRequestDelegate) fetchRequestFileAndExt(reqURI string) (string,string){
	if npos:=strings.Index(reqURI,"?");npos>0{
		reqURI = reqURI[:npos];
	}
	baseFile := filepath.Base(reqURI);
	if strings.HasSuffix(baseFile,"/"){
		baseFile = strings.TrimRight(baseFile,"/")
	}
	if baseFile == "" || baseFile == "." {
		return "","";
	}
	baseFile = strings.ToLower(baseFile);
	return baseFile,filepath.Ext(baseFile)
}
//根据返回的Content-Type来确定是否需要保留
func (this* MyRequestDelegate) checkIsValidByContentType(contentType string) bool{
	contentType = strings.ToLower(contentType);
	if strings.HasPrefix(contentType,"image/") ||
		strings.HasPrefix(contentType,"	audio/") ||
		strings.HasPrefix(contentType,"	video/"){
		return false;
	}
	if contentType == "application/octet-stream"{
		return false;
	}
	return true;
}
//保存请求和返回数据
func (this* MyRequestDelegate) saveResponseData(moduleName string,ctx *Context, resp *http.Response){
	//先根据根据长度判断一下,如果太长了就不要,可能就不是数据
	if(ctx==nil ||ctx.Req == nil || resp == nil){
		return;
	}
	if resp.StatusCode != 200{
		return;
	}
	var respLen = resp.ContentLength
	if respLen>1024*1024{
		return
	}
	//生成个新的文件
	moduleDir := filepath.Join(saveProxyDataDir,moduleName);
	if _,err := os.Stat(moduleDir);err!=nil{
		os.Mkdir(moduleDir,644);
	}
	saveFile := filepath.Join(moduleDir,time.Now().Format("20060102150405.00000")+".txt");
	var outBuffer = bytes.NewBuffer(nil);
	defer func(){
		if(outBuffer.Len()>0){
			file,err := os.Create(saveFile);
			if err == nil{
				io.Copy(file,outBuffer);
			}
			file.Close();
		}
	}()
	data,err:=httputil.DumpRequest(ctx.Req,false);
	if err == nil{
		outBuffer.Write(data);
	}else{
		outBuffer.WriteString("DUMP REQUEST ERROR:"+err.Error())
	}
	if len(ctx.textBody)>0{
		outBuffer.WriteString(ctx.textBody);
		outBuffer.WriteString("\r\n\r\n")
	}
	outBuffer.WriteString(lineSplit);
	data,err = httputil.DumpResponse(resp,false);
	if err == nil{
		outBuffer.Write(data);
	}
	outBuffer.WriteString(lineSplit);
	//处理resp.Body
	var buf = bytes.NewBuffer(nil)
	io.Copy(buf, resp.Body)
	resp.Body.Close()
	r, err := gzip.NewReader(bytes.NewBuffer(buf.Bytes()))
	if err == nil {
		io.Copy(outBuffer,r);
		r.Close()
	}else{
		outBuffer.Write(buf.Bytes())
	}
	resp.Body = ioutil.NopCloser(buf)
}