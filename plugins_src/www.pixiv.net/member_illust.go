package main

import "C"

import (
	"encoding/json"
	"fmt"
	"io"

	//	"github.com/PuerkitoBio/goquery"
	"github.com/codepiano/baren/omohan"
	"github.com/codepiano/baren/utils"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Craw struct {
	Client   *http.Client
	Total    int
	base     string
	RootDir  string
	BeginURL *url.URL
	Limit    int
}

func (p *Craw) InitCraw(client *http.Client) (omohan.Plugin, error) {
	if p == nil {
		return &Craw{
			Client: client,
		}, nil
	} else {
		return p, nil
	}
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

// 获取图片信息
func (p *Craw) getPaintingInfo(picInfoUrl string, rootDir string) ([]*omohan.Info, error) {
	html, err := utils.GetUrlContent(p.Client, picInfoUrl)
	if err != nil {
		log.Panic(err)
	}
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
		multiPicInfoPage, err := utils.GetUrlStream(p.Client, multiPicUrl)
		if err != nil {
			log.Panic(err)
		}
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
			Method: http.MethodGet,
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

func (p *Craw) ProcessPage(allPicPageUrls []string, c chan *omohan.Info, s chan string) {
	// 下载当前页的图片
	for i, picUrl := range allPicPageUrls {
		select {
		case signal := <-s:
			log.Infof("recieve stop signal, quit, signal: %e", signal)
			close(c)
			return
		default:
			paintingInfo, err := p.getPaintingInfo(picUrl, p.RootDir)
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
}

// 下载单张
func (p *Craw) fetchSingle(beginUrl string, c chan *omohan.Info, s chan string) {
	html, err := utils.GetUrlContent(p.Client, p.BeginURL.String())
	if err != nil {
		log.Panic(err)
	}
	// 获取作者
	author := regexpFirstMatchGroup("<title>「[^」]+」/「([^」]+)」", html)
	authorId := regexpFirstMatchGroup(`"authorId":"(\d+)"`, html)
	p.Total = 1
	// 拼接目录名
	existDir := p.processAuthorRename(authorId)
	if existDir == "" {
		p.RootDir = authorId + "-" + author
	} else {
		p.RootDir = existDir
	}
	allPicPageUrls := make([]string, 0)
	allPicPageUrls = append(allPicPageUrls, beginUrl)
	p.ProcessPage(allPicPageUrls, c, s)
}

// 下载全部
func (p *Craw) fetchAll(beginUrl string, c chan *omohan.Info, s chan string) {
	html, err := utils.GetUrlContent(p.Client, beginUrl)
	// 获取作者
	author := regexpFirstMatchGroup("<title>「([^」]+)」的作品", html)
	// 获取用户 id
	authorId := regexpFirstMatchGroup("id=(\\d+)", beginUrl)
	// 获取所有图片 id
	allPicAjaxUrl := fmt.Sprintf("https://www.pixiv.net/ajax/user/%s/profile/all", authorId)
	allPicBody, err := utils.GetUrlStream(p.Client, allPicAjaxUrl)
	if err != nil {
		log.Panic(err)
	}
	bytes, err := io.ReadAll(allPicBody)
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
	log.Infof("total pic %s, limit %s", p.Total, p.Limit)
	if p.Total > p.Limit {
		picNumberIds = picNumberIds[:p.Limit]
	}
	allPicPageUrls := make([]string, 0)
	// 拼接目录名
	existDir := p.processAuthorRename(authorId)
	if existDir == "" {
		p.RootDir = authorId + "-" + author
	} else {
		p.RootDir = existDir
	}
	for _, value := range picNumberIds {
		picUrl := "https://www.pixiv.net/member_illust.php?mode=medium&illust_id=" + strconv.Itoa(value)
		allPicPageUrls = append(allPicPageUrls, picUrl)
	}
	p.ProcessPage(allPicPageUrls, c, s)
}

func (p *Craw) processAuthorRename(id string) string {
	m, _ := filepath.Glob(p.base + "/" + id + "-*")
	if len(m) > 1 {
		log.Panicf("id %s match many dir!, may be duplicate!", id)
	} else if len(m) == 1 {
		log.Infof("id: %s dir already exist use this")
		return utils.RemoveStartToRightMost(m[0], '/')
	} else {
		log.Infof("first time download id: %s")
		return ""
	}
	return ""
}

func (p *Craw) Baren(beginUrl string, c chan *omohan.Info, s chan string, limit int, base string) error {
	// 判断是不是单张图片
	u, err := url.Parse(beginUrl)
	if err != nil {
		log.Panicf("parse %s error", beginUrl, err)
	}
	p.BeginURL = u
	p.Limit = limit
	p.base = base
	m, _ := url.ParseQuery(u.RawQuery)
	if m.Get("mode") == "medium" && m.Get("illust_id") != "" {
		log.Infof("fetch single", beginUrl)
		p.fetchSingle(beginUrl, c, s)
		return nil
	} else {
		log.Infof("fetch all", beginUrl)
		p.fetchAll(beginUrl, c, s)
	}
	return nil
}
