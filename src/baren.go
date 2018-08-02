package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"io"
	"net/http"
	"os"
	"plugin"
	"regexp"
	"strings"
)

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

func loadUrl(url string) io.ReadCloser {
	res, err := http.Get(url)
	if err != nil {
		log.Panicf("get page: %v", err)
	}
	if res.StatusCode != 200 {
		log.Panicf("status code error: %d %s", res.StatusCode, res.Status)
	}
	return res.Body
}

func loadPlugin(domain string, path string) interface{} {
	log.Infof("load plugin %s.so from ./plugins/%s", domain, path)
	p, err := plugin.Open("./plugins/" + domain + "/" + path + ".so")
	if err != nil {
		log.Panicf("could not load %s.so under ./plugins/%s, error: %v", path, domain, err)
	}

	initPlugin, err := p.Lookup("InitPlugin")
	if err != nil {
		log.Panicf("plugin must have function InitPlugin: %v", err)
	}
	plugin, err := initPlugin.(func() (interface{}, error))()
	if err != nil {
		log.Panicf("init plugin error: %v", err)
	}
	return plugin
}

func download(url string, path string, fileName string) int {
	// 判断目录是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			log.Panicf("create dir failed, dir: %s, %v", path, err)
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		log.Panicf("download failed, url: %s, %v", url, err)
	}
	defer resp.Body.Close()

	filePath := path + "/" + fileName
	if file, err := os.Stat(filePath); err == nil && file.Size() != 0 {
		log.Infof("file %s already exists and size is not 0", filePath)
		return 0
	} else {
		out, err := os.Create(filePath)
		if err != nil {
			log.Panicf("can create file path: %v", err)
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			log.Panicf("write to file failed, path: %s, %v", path, err)
		}
		log.Infof("file %s download success, save to %s", url, filePath)
		return 1
	}
}

func baren(url string) {
	pathRex := regexp.MustCompile("(?:https?://)?([^?]*)\\??")
	pluginPath := pathRex.FindStringSubmatch(url)
	if len(pluginPath) != 2 {
		log.Panicf("not a valid url: %s", url)
	}
	pathSplit := strings.SplitN(pluginPath[1], "/", 2)
	domain := pathSplit[0]
	path := pathSplit[1]
	plugin := loadPlugin(domain, path).(Plugin)
	body := loadUrl(url)
	resultChannel := make(chan []string, 100)
	go plugin.Baren(body, loadUrl, resultChannel)
	index := 0
	for value := range resultChannel {
		rootDir := "/home/codepiano/" + value[1] + "/"
		index = index + 1
		log.Infof("%d: %q\n", index, value)
		status := download(value[0], rootDir, value[2])
		// 下载完毕
		if status == 1 {
			content, _ := json.Marshal(value)
			appendToFile(rootDir+"/result.txt", content)
		}
	}
	defer body.Close()
}

func main() {
	initConfig()
	initFlags()
	url := viper.GetString("url")
	baren(url)
}
