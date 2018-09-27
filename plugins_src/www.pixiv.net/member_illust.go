package main

import "C"

import (
	"encoding/json"
	"fmt"
	//	"github.com/PuerkitoBio/goquery"
	"github.com/codepiano/baren/omohan"
	"github.com/codepiano/baren/utils"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Domain = "https://www.pixiv.net"
const PaintingPage = "https://www.pixiv.net/member_illust.php"

type plugin struct {
	Client   *http.Client
	Total    int
	RootDir  string
	BeginURL *url.URL
}

func InitPlugin() (f interface{}, err error) {
	f = plugin{}
	return
}

// 正则表达式匹配，返回第一个 group
func regexpFirstMatchGroup(regex string, content string) string {
	rex := regexp.MustCompile(regex)
	match := rex.FindStringSubmatch(content)
	if len(match) != 2 {
		log.Panicf("can not found match by regex: %s in %s, %v", regex, content, match)
	}
	return match[1]
}

func formatDate(original string) string {
	layout := "2006-01-02T15:04:05Z07:00"
	t, err := time.Parse(layout, original)
	if err != nil {
		log.Errorf("tran date error, original date string: %s, %v", original, err)
		return original
	}

	return t.Format("20060102.1504")
}

func getHtmlSourceCode(url string, loader func(string) io.ReadCloser) string {
	// 获取页面源码
	pageSource := loader(url)
	defer pageSource.Close()
	html, err := utils.StreamToString(pageSource)
	if err != nil {
		log.Panicf("get %s html source error", url, err)
	}
	return html
}

// 获取图片信息
func getPaintingInfo(picInfoUrl string, loader func(string) io.ReadCloser, rootDir string) ([]*omohan.Info, error) {
	html := getHtmlSourceCode(picInfoUrl, loader)
	// 获取图片数量
	paintingCountRex := regexp.MustCompile(`"pageCount":(\d+),"bookmarkCount"`)
	paintingCountMatch := paintingCountRex.FindStringSubmatch(html)
	if len(paintingCountMatch) != 2 {
		return nil, fmt.Errorf(`can not found pic count by regex: "pageCount":(\d+),"bookmarkCount"`)
	}
	pageCountNumber, err := strconv.Atoi(paintingCountMatch[1])
	if err != nil {
		log.Panicf("page count is not a number", err)
	}
	pageCount := pageCountNumber
	// 获取图片地址
	var urls []string
	if pageCount > 1 {
		multiPicUrl := strings.Replace(picInfoUrl, "medium", "manga", -1)
		multiPicInfoPage := loader(multiPicUrl)
		defer multiPicInfoPage.Close()
		multiPicPageHtml, err := utils.StreamToString(multiPicInfoPage)
		if err != nil {
			return nil, fmt.Errorf("get %s html source error", multiPicUrl, err)
		}
		multiPicUrlRex := regexp.MustCompile("data-src=\"([^\"]+)\"")
		allMatches := multiPicUrlRex.FindAllStringSubmatch(multiPicPageHtml, -1)
		if allMatches == nil || len(allMatches) != pageCount {
			return nil, fmt.Errorf("can not found pic url in %s or page number %d is wrong", multiPicUrl, pageCount)
		}
		for _, match := range allMatches {
			urls = append(urls, match[1])
		}
	} else {
		largePicUrlRex := regexp.MustCompile("\"original\":\"([^\"]+)\"")
		largePicUrlMatch := largePicUrlRex.FindStringSubmatch(html)
		if len(largePicUrlMatch) != 2 {
			return nil, fmt.Errorf("can not found pic url by regex: \"original\":\"([^\"]+)\"")
		}
		urls = []string{largePicUrlMatch[1]}
	}
	// 获取绘画名称
	paintingNameRex := regexp.MustCompile("\"illustTitle\":\"([^\"]+)\"")
	paintingNameMatch := paintingNameRex.FindStringSubmatch(html)
	if len(paintingNameMatch) != 2 {
		return nil, fmt.Errorf("can not found pic name by regex: \"illustTitle\":\"([^\"]+)\"")
	}
	paintingName, err := strconv.Unquote(`"` + utils.NormalizePath(paintingNameMatch[1]) + `"`)
	if err != nil {
		return nil, fmt.Errorf("unquote %s  failed!", paintingNameMatch[1])
	}
	// 获取绘画日期
	paintingDateRex := regexp.MustCompile("\"createDate\":\"([^\"]+)\"")
	paintingDateMatch := paintingDateRex.FindStringSubmatch(html)
	if len(paintingDateMatch) != 2 {
		return nil, fmt.Errorf("can not found pic date by regex: \"illustTitle\":\"([^\"]+)\"")
	}
	paintingDate := paintingDateMatch[1]
	// 组装 header
	header := map[string][]string{
		"referer": {picInfoUrl},
	}
	requests := make([]*omohan.Info, 0)
	for index, largePicUrl := range urls {
		largePicUrl = strings.Replace(largePicUrl, "\\", "", -1)
		fileExtension := utils.RemoveStartToRightMost(largePicUrl, '.')
		// 组装 url
		largePic, err := url.Parse(largePicUrl)
		if err != nil {
			log.Panicf("parse largePicUrl %s error: %s", largePicUrl, err)
		}
		request := &http.Request{
			Method: "GET",
			URL:    largePic,
			Header: header,
		}
		// 组装 info 对象
		fileName := fmt.Sprintf("%s_%s_%02d_%s.%s", formatDate(paintingDate), paintingName, index, utils.RemoveStartToRightMost(picInfoUrl, '='), fileExtension)
		requests = append(requests, &omohan.Info{
			Request:  request,
			FileName: fileName,
			Dir:      rootDir,
		})
	}
	return requests, nil
}

func (p plugin) ProcessPage(allPicPageUrls []string, loader func(string) io.ReadCloser, c chan *omohan.Info, s chan string) {
	// 下载当前页的图片
	for i, picUrl := range allPicPageUrls {
		select {
		case signal := <-s:
			log.Infof("recieve stop signal, quit, signal: %e", signal)
			return
		default:
			paintingInfo, err := getPaintingInfo(picUrl, loader, p.RootDir)
			if err != nil {
				log.Errorf("get painting info from %s failed: %v", picUrl, err)
			} else {
				log.Infof("process %d of %d, url:%s ", i+1, p.Total, picUrl)
				for _, info := range paintingInfo {
					c <- info
				}
			}
		}
	}
	close(c)
	close(s)
}

// 下载单张
func (p plugin) fetchSingle(beginUrl string, loader func(string) io.ReadCloser, c chan *omohan.Info, s chan string) {
	html := getHtmlSourceCode(p.BeginURL.String(), loader)
	// 获取作者
	author := regexpFirstMatchGroup("<title>「[^」]+」/「([^」]+)」", html)
	authorId := regexpFirstMatchGroup(`"authorId":"(\d+)"`, html)
	p.Total = 1
	p.RootDir = authorId + "-" + author
	allPicPageUrls := make([]string, 0)
	allPicPageUrls = append(allPicPageUrls, beginUrl)
	p.ProcessPage(allPicPageUrls, loader, c, s)
}

// 下载全部
func (p plugin) fetchAll(beginUrl string, loader func(string) io.ReadCloser, c chan *omohan.Info, s chan string) {
	html := getHtmlSourceCode(beginUrl, loader)
	// 获取作者
	author := regexpFirstMatchGroup("<title>「([^」]+)」的作品", html)
	// 获取用户 id
	authorId := regexpFirstMatchGroup("id=(\\d+)", beginUrl)
	// 获取所有图片 id
	allPicAjaxUrl := fmt.Sprintf("https://www.pixiv.net/ajax/user/%s/profile/all", authorId)
	allPicBody := loader(allPicAjaxUrl)
	bytes, err := ioutil.ReadAll(allPicBody)
	if err != nil {
		log.Panicf("can not found id by regex: id=(\\d+)")
	}
	var data map[string]interface{}
	if err := json.Unmarshal(bytes, &data); err != nil {
		log.Panicf("unmarshal string error: %s", string(bytes), err)
	}
	if data["error"] == true {
		log.Panicf("get all pic id error: %s", string(bytes), err)
	}
	allPicBodyObject := data["body"].(map[string]interface{})
	allPicId := allPicBodyObject["illusts"].(map[string]interface{})
	// 字符串类型，转换为数字并排倒序
	var picNumberIds []int
	for key, _ := range allPicId {
		number, err := strconv.Atoi(key)
		if err != nil {
			log.Panicf("atoi to number failed: %s", key, err)
		}
		picNumberIds = append(picNumberIds, number)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(picNumberIds)))
	// 总数
	p.Total = len(picNumberIds)
	log.Infof("total pic %s", p.Total)
	allPicPageUrls := make([]string, 0)
	// 拼接目录名
	p.RootDir = authorId + "-" + author
	for _, value := range picNumberIds {
		picUrl := "https://www.pixiv.net/member_illust.php?mode=medium&illust_id=" + strconv.Itoa(value)
		allPicPageUrls = append(allPicPageUrls, picUrl)
	}
	p.ProcessPage(allPicPageUrls, loader, c, s)
}

func (p plugin) Baren(beginUrl string, loader func(string) io.ReadCloser, c chan *omohan.Info, s chan string) {
	// 判断是不是单张图片
	u, err := url.Parse(beginUrl)
	if err != nil {
		log.Panicf("parse %s error", beginUrl, err)
	}
	p.BeginURL = u
	m, _ := url.ParseQuery(u.RawQuery)
	if m.Get("mode") == "medium" && m.Get("illust_id") != "" {
		log.Infof("fetch single", beginUrl)
		p.fetchSingle(beginUrl, loader, c, s)
		return
	} else {
		log.Infof("fetch all", beginUrl)
		p.fetchAll(beginUrl, loader, c, s)
	}
}
