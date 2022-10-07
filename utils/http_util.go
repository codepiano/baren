package utils

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
)

var Header = http.Header{
	"user-agent": []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36"},
}

func GetUrlStream(client *http.Client, url string) (io.ReadCloser, error) {
	res, err := client.Get(url)
	if err != nil {
		log.Infof("get page: %v", err)
		return nil, err
	}
	if res.StatusCode != 200 {
		log.Infof("status code error: %d %s", res.StatusCode, res.Status)
		return nil, errors.New("resp status error")
	}
	return res.Body, nil
}

func GetUrlContent(client *http.Client, url string) (string, error) {
	// 获取页面源码
	pageSource, err := GetUrlStream(client, url)
	if err != nil {
		return "", err
	}
	defer pageSource.Close()
	html, err := StreamToString(pageSource)
	if err != nil {
		return "", err
	}
	return html, nil
}

func GetUrlContentBytes(client *http.Client, url string) ([]byte, error) {
	// 获取页面源码
	pageSource, err := GetUrlStream(client, url)
	if err != nil {
		return nil, err
	}
	defer pageSource.Close()
	content, err := io.ReadAll(pageSource)
	if err != nil {
		return nil, err
	}
	return content, nil
}

type Downloader struct {
	Client *http.Client
}

func (d *Downloader) Download(request *http.Request, path string, fileName string) error {
	url := request.URL.String()
	err := MkdirIfNotExist(path, 0755)
	if err != nil {
		log.Errorf("mkdir error! %s", path)
		return err
	}
	filePath := path + "/" + fileName
	if file, err := os.Stat(filePath); err == nil && file.Size() != 0 {
		log.Errorf("file %s already exists and size is not 0", filePath)
		return err
	}
	resp, err := d.Client.Do(request)
	if err != nil {
		log.Errorf("download failed, url: %s, %v", url, err)
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filePath)
	if err != nil {
		log.Errorf("can create file path: %v", err)
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Panicf("write to file failed, path: %s, %v", path, err)
	}
	log.Infof("url %s download success, save to %s", url, filePath)
	return nil
}
