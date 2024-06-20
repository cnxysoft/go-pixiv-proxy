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
	params := strings.Split(c.req.URL.Path, "/")
	api := params[2]
	if api == "illust" {
		pid := strings.Split(c.req.URL.RawQuery, "=")[1]
		if _, err := strconv.Atoi(pid); err != nil {
			c.String(400, "pid invalid")
			return
		}
		GetMode = 1
		proxyHttpReq(c, "https://www.pixiv.net/ajax/illust/"+pid, "pixiv api error", reqOptions{Mode: GetMode})
	} else if api == "search" {
		query := strings.Split(c.req.URL.RawQuery, "&")
		var parm []map[string]string
		for _, q := range query {
			tmp := strings.Split(q, "=")
			m := make(map[string]string)
			if len(tmp) == 2 {
				m[tmp[0]] = tmp[1]
				parm = append(parm, m)
			}
		}
		if parm == nil {
			c.String(400, "query invalid")
			return
		}
		word := parm[0]["word"]
		if word == "" {
			c.String(400, "word invalid")
			return
		}
		page := 0.0
		reqPage := ""
		if len(parm) > 1 {
			reqPage = parm[1]["page"]
			if p, err := strconv.Atoi(reqPage); err == nil {
				reqPage = "?p=" + getTargetPage(float64(p))
				page = float64(p)
			}
		}
		GetMode = 2
		proxyHttpReq(c, "https://www.pixiv.net/ajax/search/artworks/"+word+reqPage, "pixiv api error", reqOptions{GetMode, page})
	} else if api == "user" {
		uid := params[len(params)-1]
		if _, err := strconv.Atoi(uid); err != nil {
			c.String(400, "uid invalid")
			return
		}
		GetMode = 3
		proxyHttpReq(c, "https://www.pixiv.net/ajax/user/"+uid, "pixiv api error", reqOptions{Mode: GetMode})
	} else if api == "tags" {
		tag := params[len(params)-1]
		GetMode = 4
		proxyHttpReq(c, "https://www.pixiv.net/ajax/tags/frequent/illust"+tag, "pixiv api error", reqOptions{Mode: GetMode})
	}
}

// 获取需要访问的目标页，如带Opt，则返回值包含(opt位)小鼠
func getTargetPage(page float64, opt ...int) string {
	p := math.Round(page / 2.0)
	if opt != nil {
		log.Infof("getTargetPage: %.0f", p)
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
	log.SetFormatter(&easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		LogFormat:       "[%lvl%][%time%]: %msg% \n",
	})
	log.SetLevel(log.InfoLevel)
}

func main() {
	flag.Parse()
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
