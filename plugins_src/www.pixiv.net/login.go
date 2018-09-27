package main

import "C"

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const Domain = "https://www.pixiv.net/"
const LoginPage = "https://accounts.pixiv.net/login?lang=zh&source=pc&view_type=page&ref=wwwtop_accounts_index"
const LoginApi = "https://accounts.pixiv.net/api/login?lang=zh"

type login struct{}

func InitPlugin() (f interface{}, err error) {
	f = login{}
	return
}

func (login) Login(client *http.Client, config map[string]string) {
	// 从登录页面获取 post_key 和 PHPSESSID cookie
	loginResp, err := client.Get(LoginPage)
	if err != nil {
		log.Panicf("get page: %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != 200 {
		log.Panicf("status code error: %d %s", loginResp.StatusCode, loginResp.Status)
	}
	doc, err := goquery.NewDocumentFromReader(loginResp.Body)
	postKey, exist := doc.Find("input[name=post_key]").Attr("value")
	if !exist {
		log.Panicf("no init config: input[name=post_key] in %s", LoginPage)
	}
	cookies := loginResp.Cookies()
	// 生成登录的请求体
	bodyString := fmt.Sprintf("pixiv_id=%s&captcha=&g_recaptcha_response=&password=%s&post_key=%s&source=pc&ref=wwwtop_accounts_index&return_to=https://www.pixiv.net/", config["id"], config["password"], postKey)
	fmt.Println(bodyString)
	// 请求登录接口
	req, err := http.NewRequest("POST", LoginApi, strings.NewReader(bodyString))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Panicf("login api request failed: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	// 响应 {"error":false,"message":"","body":{"success":{"return_to":"https:\/\/www.pixiv.net\/"}}}
	// 判断响应是否成功
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Panicf("unmarshal response body failed: %s", err)
	}
	if result["error"].(bool) {
		log.Panicf("login api request failed, response body: %s", result)
	}
	// 设置 cookie
	u, err := url.Parse(Domain)
	if err != nil {
		log.Panicf("parse domain %s error: %s", Domain, err)
	}
	client.Jar.SetCookies(u, resp.Cookies())
	fmt.Println(client.Jar.Cookies(u))
}
