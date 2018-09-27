package main

import "C"

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/codepiano/baren/omohan"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const Domain = "https://www.vangoghmuseum.nl"

type plugin struct {
	Client *http.Client
}

func InitPlugin() (f interface{}, err error) {
	f = plugin{}
	return
}

func formatDate(original string) string {
	layout := "January 2006"
	t, err := time.Parse(layout, original)
	if err != nil {
		log.Errorf("tran date error, original date string: %s, %v", original, err)
		return original
	}

	return t.Format("2006.01")
}

func transDate(original string) string {
	if strings.ContainsAny(original, "-") {
		dateRex := regexp.MustCompile("([a-zA-z]+)-([a-zA-z]+) (\\d{4})")
		matches := dateRex.FindStringSubmatch(original)
		if len(matches) == 0 {
			// 处理日期格式 Februry 1877-March 1878
			dates := strings.Split(original, "-")
			if len(dates) != 2 {
				log.Panicf("painting dates info is not valid: %q", dates)
			}
			return formatDate(dates[0]) + "-" + formatDate(dates[1])
		} else {
			// 处理日期格式 Februry-March 1878
			return formatDate(matches[1]+" "+matches[3]) + "-" + formatDate(matches[2]+" "+matches[3])
		}
	} else {
		// 处理日期格式 March 1878
		return formatDate(original)
	}
}

func getPaintingInfo(doc *goquery.Document) (*omohan.Info, error) {
	// 获取图片地址
	largePicUrl, exist := doc.Find("#top div.download-buttons-container > a.rounded-right").Attr("href")
	if !exist {
		return nil, fmt.Errorf("can not found selector <#top div.download-buttons-container > a.rounded-right>")
	}
	// 获取绘画信息
	paintingInfo := doc.Find("#top  div.actions p.info-title")
	if paintingInfo == nil {
		return nil, fmt.Errorf("load painting info failed! can not find selector <#top  div.actions p.info-title>")
	}
	paintingName := paintingInfo.Find("strong").Text()
	paintingAuthorAndDate := paintingInfo.Find("em").Text()
	if paintingName == "" || paintingAuthorAndDate == "" {
		return nil, fmt.Errorf("load painting info failed! can not found <em> in <#top  div.actions p.info-title>")
	}
	info := strings.Split(paintingAuthorAndDate, ", ")
	// 获取绘画地点
	// paintingAddress := doc.Find("p.text-bold").Text()
	// 获取绘画介绍
	// paintingIntroduce := doc.Find("#top > div > div.art-object-body.has-footer > div > div:nth-child(1) > div > section > article > p:nth-child(7)").Text()
	var fileName string
	if len(info) == 1 {
		fileName = paintingName
	} else {
		fileName = transDate(info[1]) + "-" + paintingName
	}

	fileExtension := largePicUrl[strings.LastIndex(largePicUrl, "."):strings.LastIndex(largePicUrl, "?size=")]
	req, _ := http.NewRequest("GET", Domain+largePicUrl, nil)
	return &omohan.Info{
		Request:  req,
		FileName: fileName + fileExtension,
		Dir:      info[0],
	}, nil
}

func (plugin) Baren(beginUrl string, loader func(string) io.ReadCloser, c chan *omohan.Info, s chan string) {
	// 获取页面源码
	pageSource := loader(beginUrl)
	defer pageSource.Close()
	doc, err := goquery.NewDocumentFromReader(pageSource)
	if err != nil {
		log.Panicf("init doc from html: %s", beginUrl, err)
	}

	elements := doc.Find("a.link-teaser")
	total := elements.Length()
	log.Infof("total pic %d", total)
	elements.Each(func(i int, s *goquery.Selection) {
		singlePageHref, _ := s.Attr("href")
		url := Domain + singlePageHref
		singleBody := loader(url)
		log.Infof("%d of %d: %s", i, total, url)
		singleDoc, err := goquery.NewDocumentFromReader(singleBody)
		if err != nil {
			log.Panicf("init doc from html: %v", err)
		}
		defer singleBody.Close()
		paintingInfo, err := getPaintingInfo(singleDoc)
		if err != nil {
			log.Errorf("get painting info from %s failed: %v", url, err)
		} else {
			c <- paintingInfo
		}
	})
	defer close(c)
	defer close(s)
}
