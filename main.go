package main

import (
	_ "embed"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	easy "github.com/t-tomalak/logrus-easy-formatter"
	"github.com/tidwall/gjson"
)

var (
	host    string
	port    string
	domain  string
	cookies string
	debug   bool
	//go:embed index.html
	indexHtml     string
	directTypes   = []string{"img-original", "img-master", "c", "user-profile", "img-zip-ugoira"}
	imgTypes      = []string{"original", "regular", "small", "thumb", "mini"}
	docExampleImg = `![regular](http://example.com/98505703?t=regular)

![small](http://example.com/98505703?t=small)

![thumb](http://example.com/98505703?t=thumb)

![mini](http://example.com/98505703?t=mini)`
)

type Illust struct {
	origUrl string
	urls    map[string]gjson.Result
}

func handlePixivProxy(rw http.ResponseWriter, req *http.Request) {
	var err error
	var realUrl string
	c := &Context{
		rw:  rw,
		req: req,
	}
	path := req.URL.Path
	log.Info(req.Method, " ", req.URL.String())
	spl := strings.Split(path, "/")[1:]
	switch spl[0] {
	case "":
		c.String(200, indexHtml)
		return
	case "favicon.ico":
		c.WriteHeader(404)
		return
	case "api":
		handleIllustInfo(c)
		return
	}
	imgType := req.URL.Query().Get("t")
	if imgType == "" {
		imgType = "original"
	}
	if !in(imgTypes, imgType) {
		c.String(400, "invalid query")
		return
	}
	if in(directTypes, spl[0]) {
		realUrl = "https://i.pximg.net" + path
	} else {
		if _, err = strconv.Atoi(spl[0]); err != nil {
			c.String(400, "invalid query")
			return
		}
		illust, err := getIllust(spl[0])
		if err != nil {
			c.String(400, "pixiv api error")
			return
		}
		if r, ok := illust.urls[imgType]; ok {
			realUrl = r.String()
		} else {
			c.String(400, "this image type not exists")
			return
		}
		if realUrl == "" {
			c.String(400, "this image needs login, set GPP_COOKIES env.")
		}
		if len(spl) > 1 {
			realUrl = strings.Replace(realUrl, "_p0", "_p"+spl[1], 1)
		}
	}
	var GetMode int = 0
	proxyHttpReq(c, realUrl, "fetch pixiv image error", reqOptions{Mode: GetMode})
}

func handleIllustInfo(c *Context) {
	var GetMode int
	Params := strings.Split(c.req.URL.Path, "/")
	api := Params[2]
	if api == "illust" {
		pid := strings.Split(c.req.URL.RawQuery, "=")[1]
		if _, err := strconv.Atoi(pid); err != nil {
			c.String(400, "pid invalid")
			return
		}
		GetMode = 1
		proxyHttpReq(c, "https://www.pixiv.net/ajax/illust/"+pid, "pixiv api error", reqOptions{Mode: GetMode})
	} else if api == "search" {
		parms := getParams(c.req.URL.RawQuery)
		if parms == nil {
			c.String(400, "query invalid")
			return
		}
		word := parms["word"]
		if word == "" {
			c.String(400, "word invalid")
			return
		}
		page := 0.0
		reqPage := parms["page"]
		if reqPage != "" {
			if p, err := strconv.Atoi(reqPage); err == nil {
				reqPage = "?p=" + getTargetPage(float64(p))
				page = float64(p)
			}
		}
		GetMode = 2
		proxyHttpReq(c, "https://www.pixiv.net/ajax/search/artworks/"+word+reqPage, "pixiv api error", reqOptions{GetMode, page})
	} else if api == "search_user" {
		params := getParams(c.req.URL.RawQuery)
		if params["word"] == "" {
			c.String(400, "uid or name invalid")
			return
		}
		uid := params["word"]
		GetMode = 3
		proxyHttpReq(c, "https://www.pixiv.net/ajax/user/"+uid, "pixiv api error", reqOptions{Mode: GetMode})
	} else if api == "tags" {
		tag := Params[len(Params)-1]
		GetMode = 4
		proxyHttpReq(c, "https://www.pixiv.net/ajax/tags/frequent/illust"+tag, "pixiv api error", reqOptions{Mode: GetMode})
	} else if api == "rank" {
		params := getParams(c.req.URL.RawQuery)
		if params == nil {
			c.String(400, "query invalid")
			return
		}
		mode := params["mode"]
		content := params["content"]
		if mode != "" {
			Mode := "daily"
			if strings.Contains(mode, "_manga") {
				mode = strings.ReplaceAll(mode, "_manga", "")
				content = "&content=manga"
			}
			if strings.Contains(mode, "male") || strings.Contains(mode, "female") {
				Mode = strings.ReplaceAll(mode, "day_", "")
			} else if strings.Contains(mode, "original") {
				Mode = "original"
			} else if strings.Contains(mode, "rookie") {
				Mode = "rookie"
			} else if strings.Contains(mode, "day") {
				Mode = strings.ReplaceAll(mode, "day", "daily")
			} else if strings.Contains(mode, "week") {
				if !strings.Contains(mode, "weekly") {
					Mode = strings.ReplaceAll(mode, "week", "weekly")
				}
			} else if strings.Contains(mode, "month") {
				if !strings.Contains(mode, "monthly") {
					Mode = strings.ReplaceAll(mode, "month", "monthly")
				}
			}
			mode = "&mode=" + Mode
		}
		date := params["date"]
		if date != "" {
			if strings.Contains(date, "-") {
				date = strings.ReplaceAll(date, "-", "")
			}
			date = "&date=" + date
		}
		if content != "" {
			content = "&content=" + content
		}
		page := 0.0
		reqPage := params["page"]
		if reqPage != "" {
			p, err := strconv.Atoi(reqPage)
			if err != nil {
				c.String(400, "page invalid")
				return
			}
			reqPage = "&p=" + getTargetPage(float64(p))
			page = float64(p)
		}
		GetMode = 5
		proxyHttpReq(c, "https://www.pixiv.net/ranking.php?format=json"+mode+date+content+reqPage, "pixiv api error", reqOptions{GetMode, page})
	} else if api == "member_illust" {
		params := getParams(c.req.URL.RawQuery)
		if params == nil {
			c.String(400, "query invalid")
			return
		}
		uid := params["id"]
		if uid == "" {
			c.String(400, "word invalid")
			return
		}
		page := 0.0
		reqPage := params["page"]
		if reqPage != "" {
			if p, err := strconv.Atoi(reqPage); err == nil {
				// reqPage = "?p=" + getTargetPage(float64(p))
				page = float64(p)
			}
		}
		GetMode = 6
		proxyHttpReq(c, "https://www.pixiv.net/ajax/user/"+uid+"/profile/all", "pixiv api error", reqOptions{GetMode, page})
	}
}

func getParams(rawQuery string) map[string]string {
	query := strings.Split(rawQuery, "&")
	parms := make(map[string]string, len(query))
	for _, q := range query {
		t := strings.Split(q, "=")
		if len(t) == 2 {
			parms[t[0]] = t[1]
		}
	}
	return parms
}

// 获取需要访问的目标页，如带Opt，则返回值包含(opt位)小鼠
func getTargetPage(page float64, opt ...int) string {
	p := math.Round(page / 2.0)
	if opt != nil {
		log.Debugf("getTargetPage: %.0f", p)
		return strconv.FormatFloat(float64(page)/2.0, 'f', opt[0], 64)
	} else {
		return strconv.FormatFloat(p, 'f', -1, 64)
	}
}

func getIllust(pid string) (*Illust, error) {
	b, err := httpGetBytes("https://www.pixiv.net/ajax/illust/" + pid)
	if err != nil {
		return nil, err
	}
	g := gjson.ParseBytes(b)
	imgUrl := g.Get("body.urls.original").String()
	return &Illust{
		origUrl: imgUrl,
		urls:    g.Get("body.urls").Map(),
	}, nil
}

func in(orig []string, str string) bool {
	for _, b := range orig {
		if b == str {
			return true
		}
	}
	return false
}

func checkEnv() {
	if os.Getenv("GPP_HOST") != "" {
		host = os.Getenv("GPP_HOST")
	}
	if os.Getenv("GPP_PORT") != "" {
		port = os.Getenv("GPP_PORT")
	}
	if os.Getenv("GPP_DOMAIN") != "" {
		domain = os.Getenv("GPP_DOMAIN")
	}
	if os.Getenv("GPP_COOKIES") != "" {
		cookies = os.Getenv("GPP_COOKIES")
	}
}

func init() {
	flag.StringVar(&host, "h", "127.0.0.1", "host")
	flag.StringVar(&port, "p", "18090", "port")
	flag.StringVar(&domain, "d", "", "your domain")
	flag.StringVar(&cookies, "c", "", "cookie")
	flag.BoolVar(&debug, "debug", false, "debug mode")
	log.SetFormatter(&easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		LogFormat:       "[%lvl%][%time%]: %msg% \n",
	})
	log.SetLevel(log.InfoLevel)
}

func main() {
	flag.Parse()
	if debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("debug mode enabled")
	}
	checkEnv()
	if domain != "" {
		indexHtml = strings.ReplaceAll(indexHtml, "{image-examples}", docExampleImg)
		indexHtml = strings.ReplaceAll(indexHtml, "http://example.com", domain)
	}
	http.HandleFunc("/", handlePixivProxy)
	log.Infof("started: %s:%s %s", host, port, domain)
	err := http.ListenAndServe(fmt.Sprintf("%s:%s", host, port), nil)
	if err != nil {
		log.Error("start failed: ", err)
	}
}
