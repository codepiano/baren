package main

import (
	"bufio"
	//"encoding/json"
	"fmt"
	"github.com/codepiano/baren/omohan"
	"github.com/codepiano/baren/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"plugin"
	"regexp"
	"strings"
	"time"
)

var Client *http.Client
var downloadCount = 0
var actualDownloadCount = 0
var craw omohan.Plugin

func initConfig() {
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Panicf("load config file: %v", err)
	}
}

func initFlags() {
	pflag.String("url", "", "web page url")
	pflag.Bool("login", false, "need login?")
	pflag.Bool("force", false, "force process all image")
	pflag.Int("limit", 1000000, "limit max download")
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
}

func appendToFile(path string, data []byte) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Panicf("failed to create file: %s, %v", path, err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	if _, err := fmt.Fprintln(w, string(data[:])); err != nil {
		log.Panicf("failed to append data to file: %s, %v", path, err)
	}
	w.Flush()
}

func initHttpClient() {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		Proxy:              http.ProxyFromEnvironment,
	}
	cookieJar, _ := cookiejar.New(nil)
	Client = &http.Client{
		Transport: tr,
		Jar:       cookieJar,
	}
}

func loadPlugin(domain string, path string) interface{} {
	log.Infof("load plugin %s.so from ./plugins/%s", path, domain)
	p, err := plugin.Open("./plugins/" + domain + "/" + path + ".so")
	if err != nil {
		log.Panicf("could not load %s.so under ./plugins/%s, error: %v", path, domain, err)
	}

	InitCraw, err := p.Lookup("InitCraw")
	if err != nil {
		log.Panicf("plugin must have function InitCraw: %v", err)
	}
	plugin, err := InitCraw.(func() (interface{}, error))()
	if err != nil {
		log.Panicf("init plugin error: %v", err)
	}
	return plugin
}

func download(request *http.Request, path string, fileName string) int {
	downloadCount = downloadCount + 1
	url := request.URL.String()
	err := utils.MkdirIfNotExist(path, 0755)
	if err != nil {
		log.Panicf("mkdir error! %s", path)
	}
	filePath := path + "/" + fileName
	if file, err := os.Stat(filePath); err == nil && file.Size() != 0 {
		log.Infof("file %s already exists and size is not 0", filePath)
		return 0
	}
	actualDownloadCount = actualDownloadCount + 1
	resp, err := Client.Do(request)
	if err != nil {
		log.Panicf("download failed, url: %s, %v", url, err)
	}
	defer resp.Body.Close()

	out, err := os.Create(filePath)
	if err != nil {
		log.Panicf("can create file path: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Panicf("write to file failed, path: %s, %v", path, err)
	}
	log.Infof("[%d/%d] file %s download success, save to %s", actualDownloadCount, downloadCount, url, filePath)
	return actualDownloadCount
}

func baren(url string, isLogin bool) {
	// 提取不带参数的 url
	pathRex := regexp.MustCompile("(?:https?://)?([^?]*)\\??")
	pluginPath := pathRex.FindStringSubmatch(url)
	if len(pluginPath) != 2 {
		log.Panicf("not a valid url: %s", url)
	}
	// 拆分 domain 和 path
	pathSplit := strings.SplitN(pluginPath[1], "/", 2)
	domain := pathSplit[0]
	// 移除 path 中的点号，例如 'pic.php'，作为对应的插件的路径
	path := utils.RemoveLeftMostToEnd(pathSplit[1], '.')
	var config map[string]string
	if isLogin {
		login := loadPlugin(domain, "login").(omohan.Login)
		if !viper.IsSet(domain) {
			log.Panicf("domain %s need login but no login config", domain)
		}
		config = viper.GetStringMapString(domain)
		login.Login(Client, config)
	}
	craw = loadPlugin(domain, path).(omohan.Plugin)
	resultChannel := make(chan *omohan.Info, 100)
	signalChannel := make(chan string, 10)
	defer close(signalChannel)
	limit := viper.GetInt("limit")
	root := config["root-dir"]
	if root == "" {
		root = viper.GetString("app.root-dir")
		if root == "" {
			log.Panicf("no root dir")
		}
		root = fmt.Sprintf("%s/%s", root, domain)
	}
	go craw.Baren(url, resultChannel, signalChannel, limit, config["root-dir"])
	force := viper.GetBool("force")
	for value := range resultChannel {
		rootDir := config["root-dir"] + "/" + value.Dir + "/"
		status := download(value.Request, rootDir, value.FileName)
		// 下载完毕
		if status > 1 {
			picInfo := fmt.Sprintf("%s, %s", value.String(), time.Now().Format("2006-01-02 15:04:05"))
			appendToFile(rootDir+"/result.txt", []byte(picInfo))
		} else if !force && status < 1 {
			signalChannel <- "stop"
			break
		}
	}
	signalChannel <- "stop"
}

func main() {
	initConfig()
	initFlags()
	initHttpClient()
	// 抓取的 url
	url := viper.GetString("url")
	// 是否需要登录
	login := viper.GetBool("login")
	root := viper.GetString("app.root")
	fmt.Println(root)
	baren(url, login)
}
