package dl

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mocukie/megalink/pkg/mega"
	"github.com/mocukie/megalink/web"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var rangeRegex = regexp.MustCompile("^bytes=(\\d+)-(\\d*)$")
var megaClient = web.MegaClient

type routerImpl struct{}

func NewRouter() web.IRouter {
	return routerImpl{}
}

func (r routerImpl) Setup(g gin.IRouter) {
	g = g.Group("/dl")
	g.Group("/:link").
		HEAD("", parseFileLink).
		GET("", parseFileLink, download)
	g.Group("/:link/file/:handle").
		HEAD("", parseFolderFileLink).
		GET("", parseFolderFileLink, download)
}

func parseFileLink(c *gin.Context) {
	link := c.Param("link")
	var g []string
	for _, r := range web.FileLinkRegexs {
		g = r.FindStringSubmatch(link)
		if len(g) > 0 {
			break
		}
	}
	if len(g) == 0 {
		c.AbortWithStatus(404)
		return
	}

	info, err := megaClient.GetPublicFileNodeInfo(g[1], g[1], g[2])
	if err != nil {
		abortWithError(c, err)
		return
	}

	if info.Attr.Name != "" {
		c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+strings.ReplaceAll(url.QueryEscape(info.Attr.Name), "+", "%20"))
	}

	c.Set("info", info)
	c.Next()
}

func parseFolderFileLink(c *gin.Context) {
	link := c.Param("link")
	handle := c.Param("handle")
	if len(handle) != mega.HandleLen || !web.FolderLinkRegex.MatchString(link) {
		c.AbortWithStatus(404)
		return
	}

	g := strings.Split(link, "!")
	fm, err := megaClient.OpenPublicFolder(g[0], g[1])
	if err != nil {
		abortWithError(c, err)
		return
	}

	node := fm.Lookup(handle)
	if node == nil {
		c.AbortWithStatus(404)
		return
	}

	info, err := fm.GetFileNodeInfo(node)
	if err != nil {
		abortWithError(c, err)
		return
	}

	if info.Attr.Name != "" {
		c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+strings.ReplaceAll(url.QueryEscape(info.Attr.Name), "+", "%20"))
	}

	c.Set("info", info)
	c.Next()
}

func download(c *gin.Context) {
	var err error
	info := c.MustGet("info").(*mega.NodeInfo)

	opt := mega.NewDownloadOption()
	if r := c.GetHeader("Range"); r != "" {
		g := rangeRegex.FindStringSubmatch(r)
		if len(g) == 0 {
			c.AbortWithStatus(http.StatusRequestedRangeNotSatisfiable)
			return
		}

		s, _ := strconv.Atoi(g[1])
		e := -1
		if g[2] != "" {
			e, _ = strconv.Atoi(g[2])
		}
		opt = opt.Range(int64(s), int64(e))
	}

	for _, k := range []string{
		"If-None-Match",
		"If-Modified-Since",
		"If-Range",
		"User-Agent",
	} {
		if v := c.GetHeader(k); v != "" {
			opt.HttpHeader(k, v)
		}
	}

	dl, err := megaClient.Download(info, opt)
	if err != nil {
		abortWithError(c, err)
		return
	}
	defer dl.Close()

	for _, k := range []string{
		"Date",
		"ETag",
		"Expires",
		"Last-Modified",
	} {
		if v := dl.Http.Header.Get(k); v != "" {
			c.Header(k, v)
		}
	}

	if dl.Http.StatusCode == http.StatusPartialContent {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", dl.Range.S, dl.Range.E, dl.Range.Total))
	}

	mimeType := mime.TypeByExtension(path.Ext(info.Attr.Name))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	c.DataFromReader(dl.Http.StatusCode, dl.Range.E-dl.Range.S+1, mimeType, dl, nil)
}

func abortWithError(c *gin.Context, err error) (code int) {
	err, code, msg := web.ConvertError(err)
	c.Abort()
	c.Error(err)
	c.String(code, msg)
	return
}
