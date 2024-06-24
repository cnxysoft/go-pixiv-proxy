package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	headers = map[string]string{
		"Referer":    "https://www.pixiv.net",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.113 Safari/537.36",
	}
	client = &http.Client{
		Transport: &http.Transport{
			Proxy: func(request *http.Request) (u *url.URL, e error) {
				return http.ProxyFromEnvironment(request)
			},
		},
	}
)

type Context struct {
	rw  http.ResponseWriter
	req *http.Request
}

type illustRespsonse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Body    struct {
		ID            string `json:"id"`
		Title         string `json:"title"`
		CreateDate    string `json:"createDate"`
		UserId        string `json:"userId"`
		UserName      string `json:"userName"`
		BookmarkCount int64  `json:"bookmarkCount"`
		ViewCount     int64  `json:"viewCount"`
		AiType        int    `json:"aiType"`
		XRestrict     int    `json:"xRestrict"`
		Tags          struct {
			Tags []struct {
				Tag       string `json:"tag"`
				Locked    bool   `json:"locked"`
				Deletable bool   `json:"deletable"`
				UserId    string `json:"userId"`
				UserName  string `json:"userName"`
			} `json:"tags"`
		} `json:"tags"`
		Urls struct {
			Original string `json:"original"`
			Regular  string `json:"regular"`
			Small    string `json:"small"`
			Thumb    string `json:"thumb"`
			Mini     string `json:"mini"`
		} `json:"urls"`
		PageCount int `json:"pageCount"`
	}
}

type pagesResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Body    []struct {
		Urls struct {
			Original  string `json:"original"`
			Regular   string `json:"regular"`
			Small     string `json:"small"`
			ThumbMini string `json:"thumb_mini"`
		} `json:"urls"`
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"body"`
}

type searchResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Body    struct {
		IllustManga struct {
			Total    int64 `json:"total"`
			LastPage int   `json:"last_page"`
			Data     []struct {
				Id        string   `json:"id"`
				Title     string   `json:"title"`
				XRestrict int      `json:"xRestrict"`
				Url       string   `json:"url"`
				Tags      []string `json:"tags"`
				UserId    string   `json:"userId"`
				UserName  string   `json:"userName"`
			} `json:"data"`
		} `json:"illustManga"`
	} `json:"body"`
}

type rankingsResponse struct {
	Contents  []rankingContents `json:"contents"`
	Mode      string            `json:"mode"`
	Content   string            `json:"content"`
	Page      int               `json:"page"`
	Prev      any               `json:"prev"`
	Next      any               `json:"next"`
	Date      string            `json:"date"`
	PrevDate  string            `json:"prev_date"`
	NextDate  any               `json:"next_date"`
	RankTotal int               `json:"rank_total"`
}

type rankingContents struct {
	Title             string   `json:"title"`
	Date              string   `json:"date"`
	Tags              []string `json:"tags"`
	Url               string   `json:"url"`
	IllustType        string   `json:"illust_type"`
	IllustBookStyle   string   `json:"illust_book_style"`
	IllustPageCount   string   `json:"illust_page_count"`
	UserName          string   `json:"user_name"`
	ProfileImg        string   `json:"profile_img"`
	IllustContentType struct {
		Sexual     int  `json:"sexual"`
		Lo         bool `json:"lo"`
		Grotesque  bool `json:"grotesque"`
		Violent    bool `json:"violent"`
		HomoSexual bool `json:"homosexual"`
		Drug       bool `json:"drug"`
		Thoughts   bool `json:"thoughts"`
		Antisocial bool `json:"antisocial"`
		Religion   bool `json:"religion"`
		Original   bool `json:"original"`
		Furry      bool `json:"furry"`
		Bl         bool `json:"bl"`
		Yuri       bool `json:"yuri"`
	} `json:"illust_content_type"`
	IllustSeries          any    `json:"illust_series"`
	IllustId              int64  `json:"illust_id"`
	Width                 int    `json:"width"`
	Height                int    `json:"height"`
	UserId                int64  `json:"user_id"`
	Rank                  int    `json:"rank"`
	YesRank               int    `json:"yes_rank"`
	RatingCount           int    `json:"rating_count"`
	ViewCount             int    `json:"view_count"`
	IllustUploadTimestamp int64  `json:"illust_upload_timestamp"`
	Attr                  string `json:"attr"`
	IsBookmarked          bool   `json:"is_bookmarked"`
	Bookmarkable          bool   `json:"bookmarkable"`
}

type reqOptions struct {
	Mode int
	Page float64
}

func (c *Context) write(b []byte, status int) {
	// c.WriteHeader(status)
	_ = status
	_, err := c.rw.Write(b)
	if err != nil {
		log.Error(err)
	}
}

func (c *Context) String(status int, s string) {
	c.write([]byte(s), status)
}

func (c *Context) WriteHeader(statusCode int) {
	c.rw.WriteHeader(statusCode)
}

func proxyHttpReq(c *Context, url string, errMsg string, options ...reqOptions) {
	mode := 0
	page := 0.0
	for _, opt := range options {
		mode = opt.Mode
		page = opt.Page
	}

	resp, err := httpGet(url)
	if err != nil {
		c.String(500, errMsg)
		return
	}
	defer resp.Body.Close()
	copyHeader(c.rw.Header(), resp.Header)
	resp.Header.Del("Cookie")
	resp.Header.Del("Set-Cookie")
	if mode == 1 {
		c.write(c.GetArtWorkInfo(resp, url, errMsg), 200)
	} else if mode == 2 {
		c.write(c.GetSearchResults(resp, url, errMsg, page), 200)
	} else if mode == 3 {
		// TODO User
	} else if mode == 4 {
		// TODO Tags
	} else if mode == 5 {
		c.write(c.GetRankingResults(resp, url, errMsg, page), 200)
	} else {
		_, _ = io.Copy(c.rw, resp.Body)
	}
}

func (c *Context) GetRankingResults(resp *http.Response, url string, errMsg string, optPage ...float64) []byte {
	p, err := io.ReadAll(resp.Body)
	if err != nil {
		c.String(500, errMsg)
		return nil
	}
	var rankings rankingsResponse
	err = json.Unmarshal(p, &rankings)
	if err != nil {
		c.String(500, err.Error())
		return nil
	}
	startNum := 0
	endNum := len(rankings.Contents)
	if endNum == 0 {
		endNum = 0
		log.Debug("no results")
	} else {
		if optPage != nil {
			startNum, endNum = getTargetPageRange(optPage[0], endNum)
			log.Debugf("startNum: %d, endNum: %d", startNum, endNum)
		}
	}

	var illust []map[string]interface{}
	for i := startNum; i < endNum; i++ {
		data := rankings.Contents[i]
		var tags []map[string]string
		for j := 0; j < len(data.Tags); j++ {
			var tag = map[string]string{
				"tag": data.Tags[j],
			}
			tags = append(tags, tag)
		}
		var illustData = map[string]interface{}{
			"id":         data.IllustId,
			"title":      data.Title,
			"x_restrict": data.IllustContentType.Sexual,
			"meta_single_page": map[string]interface{}{
				"image_urls": map[string]string{
					"original": data.Url,
				},
			},
			"image_urls": map[string]string{
				"large": data.Url,
			},
			"tags": tags,
			"user": map[string]string{
				"id":   strconv.FormatInt(data.UserId, 10),
				"name": data.UserName,
			},
			"total_bookmarks": 0,
		}
		illust = append(illust, illustData)
	}
	var ret = map[string]interface{}{
		"illusts": illust,
		"length":  len(illust), // len(searchResults.Body.IllustManga.Data)
	}
	p, err = json.Marshal(ret)
	if err != nil {
		c.String(500, errMsg)
		return nil
	}
	return p
}

func (c *Context) GetArtWorkInfo(resp *http.Response, url string, errMsg string) []byte {
	p, err := io.ReadAll(resp.Body)
	if err != nil {
		c.String(500, errMsg)
		return nil
	}
	var illust illustRespsonse
	err = json.Unmarshal(p, &illust)
	if err != nil {
		c.String(500, errMsg)
		return nil
	}
	if illust.Error {
		c.String(500, fmt.Sprintf("pixiv api error: %s", illust.Message))
		return nil
	}
	var tags []map[string]interface{}
	for i := 0; i < len(illust.Body.Tags.Tags); i++ {
		tagMap := map[string]interface{}{
			"tag": illust.Body.Tags.Tags[i].Tag,
		}
		tags = append(tags, tagMap)
	}
	var ret = map[string]interface{}{
		"illust": map[string]interface{}{
			"id":              illust.Body.ID,
			"title":           illust.Body.Title,
			"create_date":     illust.Body.CreateDate,
			"total_bookmarks": illust.Body.BookmarkCount,
			"total_view":      illust.Body.ViewCount,
			"illust_ai_type":  illust.Body.AiType,
			"x_restrict":      illust.Body.XRestrict,
			"tags":            tags,
			"length":          illust.Body.PageCount,
			"user": map[string]string{
				"id":   illust.Body.UserId,
				"name": illust.Body.UserName,
			},
		},
	}
	if illust.Body.PageCount == 1 {
		singleData := map[string]interface{}{
			"meta_single_page": map[string]string{
				"original_image_url": illust.Body.Urls.Original,
			},
		}
		ret["illust"].(map[string]interface{})["meta_single_page"] = singleData["meta_single_page"]
	} else {
		resp, err := httpGet(url + "/pages")
		if err != nil {
			c.String(500, errMsg)
			return nil
		}
		defer resp.Body.Close()
		copyHeader(c.rw.Header(), resp.Header)
		resp.Header.Del("Cookie")
		resp.Header.Del("Set-Cookie")
		p, err := io.ReadAll(resp.Body)
		if err != nil {
			c.String(500, errMsg)
			return nil
		}
		var pages pagesResponse
		err = json.Unmarshal(p, &pages)
		if err != nil {
			c.String(500, errMsg)
			return nil
		}
		if pages.Error {
			c.String(500, fmt.Sprintf("pixiv api error: %s", pages.Message))
			return nil
		}
		metaData := map[string]interface{}{
			"meta_pages": []map[string]interface{}{},
		}
		for i := 0; i < illust.Body.PageCount; i++ {
			metaData["meta_pages"] = append(metaData["meta_pages"].([]map[string]interface{}), map[string]interface{}{
				"image_urls": map[string]interface{}{
					"original": pages.Body[i].Urls.Original,
				},
			})
		}
		ret["illust"].(map[string]interface{})["meta_pages"] = metaData["meta_pages"]
	}
	p, err = json.Marshal(ret)
	if err != nil {
		c.String(500, errMsg)
		return nil
	}
	return p
}

func getTargetPageRange(page float64, total int) (int, int) {
	if page == 0 {
		page = 1
	}
	tmp := getTargetPage(page, 1)
	tmpArr := strings.Split(tmp, ".")
	var s, e int
	if len(tmpArr) == 2 {
		if tmpArr[1] == "5" {
			s = 0
			if total < 30 {
				e = total
			} else {
				e = 30
			}
		} else {
			s = 30
			e = total
		}
	} else {
		s = 0
		e = total
	}
	return s, e
}

func (c *Context) GetSearchResults(resp *http.Response, url string, errMsg string, optPage ...float64) []byte {
	p, err := io.ReadAll(resp.Body)
	if err != nil {
		c.String(500, errMsg)
		return nil
	}
	var searchResults searchResponse
	err = json.Unmarshal(p, &searchResults)
	if err != nil {
		c.String(500, errMsg)
		return nil
	}
	if searchResults.Error {
		c.String(500, fmt.Sprintf("pixiv api error: %s", searchResults.Message))
		return nil
	}
	startNum := 0
	endNum := len(searchResults.Body.IllustManga.Data)
	if endNum == 0 {
		endNum = 0
		log.Debug("no results")
	} else {
		if optPage != nil {
			startNum, endNum = getTargetPageRange(optPage[0], endNum)
			log.Debugf("startNum: %d, endNum: %d", startNum, endNum)
		}
	}

	// 取消使用 len(searchResults.Body.IllustManga.Data)
	var illust []map[string]interface{}
	for i := startNum; i < endNum; i++ {
		data := searchResults.Body.IllustManga.Data[i]
		var tags []map[string]string
		for j := 0; j < len(data.Tags); j++ {
			var tag = map[string]string{
				"tag": data.Tags[j],
			}
			tags = append(tags, tag)
		}
		var illustData = map[string]interface{}{
			"id":         data.Id,
			"title":      data.Title,
			"x_restrict": data.XRestrict,
			"meta_single_page": map[string]interface{}{
				"image_urls": map[string]string{
					"original": data.Url,
				},
			},
			"image_urls": map[string]string{
				"large": data.Url,
			},
			"tags": tags,
			"user": map[string]string{
				"id":   data.UserId,
				"name": data.UserName,
			},
			"total_bookmarks": 0,
		}
		illust = append(illust, illustData)
	}
	var ret = map[string]interface{}{
		"illusts": illust,
		"length":  len(illust), // len(searchResults.Body.IllustManga.Data)
	}
	p, err = json.Marshal(ret)
	if err != nil {
		c.String(500, errMsg)
		return nil
	}
	return p
}

func httpGet(u string) (*http.Response, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Cookie", cookies)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func httpGetReadCloser(u string) (io.ReadCloser, error) {
	resp, err := httpGet(u)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func httpGetBytes(url string) ([]byte, error) {
	body, err := httpGetReadCloser(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	b, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
