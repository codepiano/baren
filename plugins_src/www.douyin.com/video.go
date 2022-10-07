package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codepiano/baren/omohan"
	"github.com/codepiano/baren/utils"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	DouyinItemInfoAPI = `https://www.iesdouyin.com/web/api/v2/aweme/iteminfo/?item_ids=%s`
	Douyin1080InfoAPI = `https://aweme.snssdk.com/aweme/v1/play/?video_id=%s&radio=1080p&line=0`
)

type Craw struct {
	Client     *http.Client
	downloader *utils.Downloader
	Total      int
	base       string
	RootDir    string
	Login      bool
}

type VideoInfo struct {
	ItemList []*ItemInfo `json:"item_list"`
}

type ItemInfo struct {
	AwemeId     string       `json:"aweme_id"`
	Desc        string       `json:"desc"`
	CreateTime  int64        `json:"create_time"`
	Author      *Author      `json:"author"`
	Music       *Music       `json:"music"`
	Video       *Video       `json:"video"`
	VideoLabels *VideoLabels `json:"video_labels"`
	Duration    uint         `json:"duration"`
	AwemeType   int          `json:"aweme_type"`
}

type Author struct {
	UID          string        `json:"uid"`
	UniqueId     string        `json:"unique_id"`
	ShortId      string        `json:"short_id"`
	Nickname     string        `json:"nickname"`
	Signature    string        `json:"signature"`
	AvatarLarger *AvatarLarger `json:"avatar_larger"`
	FollowStatus int           `json:"follow_status"`
}

type AvatarLarger struct {
	URI     string   `json:"uri"`
	UrlList []string `json:"url_list"`
}

type Music struct {
	ID       uint   `json:"id"`
	Mid      string `json:"mid"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Duration int    `json:"duration"`
}

type Video struct {
	PlayAddr     *PlayAddr `json:"play_addr"`
	Vid          string    `json:"vid"`
	Height       int       `json:"height"`
	Width        int       `json:"width"`
	Ratio        string    `json:"ratio"`
	HasWaterMark bool      `json:"has_watermark"`
	Duration     int       `json:"duration"`
	IsLongVideo  int8      `json:"is_long_video"`
}

type PlayAddr struct {
	URI     string   `json:"uri"`
	UrlList []string `json:"url_list"`
}

type VideoLabels struct {
}

func (p *Craw) InitCraw(client *http.Client) (omohan.Plugin, error) {
	if p == nil {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		downloader := &utils.Downloader{Client: client}
		return &Craw{
			Client:     client,
			downloader: downloader,
		}, nil
	} else {
		return p, nil
	}
}

var videoIdRex = regexp.MustCompile(`www.douyin.com/video/(\d+)`)

type Info struct {
	VideoInfo        *VideoInfo `json:"video_info"`
	Video1080        string     `json:"video_1080"`
	WaterMarkVideo   string     `json:"water_mark_video"`
	NoWaterMarkVideo string     `json:"no_water_mark_video"`
}

func (p *Craw) Baren(beginUrl string, c chan *omohan.Info, s chan string, limit int, base string) error {
	idMatch := videoIdRex.FindStringSubmatch(beginUrl)
	if len(idMatch) != 2 {
		log.Panicf("illgeal video url, no id matched")
	}
	id := idMatch[1]
	fmt.Println(id)
	videoInfo, err := p.getDouyinVideoInfo(id)
	if err != nil {
		return err
	}
	info := &Info{
		VideoInfo: videoInfo,
	}
	// 优先下载 1080p 视频
	vid := ""
	if len(videoInfo.ItemList) == 0 {
		return errors.New("no vide item info")
	}
	videoItem := videoInfo.ItemList[0]
	vid = videoItem.Video.Vid
	if vid == "" {
		return errors.New("no video id")
	}
	info.Video1080, err = p.get1080URI(vid)
	if err != nil {
		return err
	}
	// 文件名
	videoDesc := utils.RemoveIllegalFileNameChars(utils.CleanLabelText(videoItem.Desc))
	if len(videoDesc) > 100 {
		videoDesc = videoDesc[0:100]
	}
	fileName := fmt.Sprintf("%s_%s_1080p", time.UnixMilli(videoItem.CreateTime).Format(utils.TimeFormat), videoDesc)
	// 先尝试下载 1080p 版，报 403，登录完再试试能不能下载
	if p.Login && info.Video1080 != "" {
		videoDownloadLink, err := url.Parse(info.Video1080)
		if err != nil {
			log.Errorf("illgeal link: %s", info.Video1080)
			return errors.New("video link illegal")
		}
		request := &http.Request{
			Method: http.MethodGet,
			URL:    videoDownloadLink,
			Header: utils.Header,
		}
		err = p.downloader.Download(request, "", fileName)
		if err != nil {
			return err
		}
		return nil
	}
	// 尝试下载无水印版视频
	if videoItem.Video != nil && videoItem.Video.PlayAddr != nil &&
		len(videoItem.Video.PlayAddr.UrlList) > 0 {
		info.WaterMarkVideo = videoItem.Video.PlayAddr.UrlList[0]
	}
	return nil
}

func (p *Craw) get1080URI(vid string) (string, error) {
	video1080 := fmt.Sprintf(Douyin1080InfoAPI, vid)
	resp, err := p.Client.Get(video1080)
	if err != nil {
		return "", err
	}
	headers := resp.Header
	if url, ok := headers["Location"]; ok {
		return url[0], nil
	}
	return "", nil
}

func (p *Craw) getNoWaterMarkURI(url string) (string, error) {
	noWaterMarkVideo := strings.ReplaceAll(url, "playwm", "play")
	resp, err := p.Client.Get(noWaterMarkVideo)
	if err != nil {
		return "", err
	}
	headers := resp.Header
	if url, ok := headers["Location"]; ok {
		return url[0], nil
	}
	return "", nil
}

func (p *Craw) getDouyinVideoInfo(id string) (*VideoInfo, error) {
	url := fmt.Sprintf(DouyinItemInfoAPI, id)
	content, err := utils.GetUrlContentBytes(p.Client, url)
	if err != nil {
		return nil, err
	}
	info := &VideoInfo{}
	err = json.Unmarshal(content, info)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func main() {
	c := &Craw{
		Client: &http.Client{},
	}
	c.Baren("https://www.douyin.com/video/7149027119568342302", nil, nil, 0, "")
}
