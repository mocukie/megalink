package static

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/mocukie/megalink/web"
	"net/http"
	"os"
	"path"
	"strings"
)

type routerImpl struct {
	prefix string
	fs     http.FileSystem
}

func NewRouter(prefix string, fs http.FileSystem) web.IRouter {
	if prefix[0] != '/' {
		panic(errors.New("web.static.NewRouter prefix must start with '/'"))
	}
	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	return routerImpl{
		prefix: prefix,
		fs:     fs,
	}
}

func (r routerImpl) Setup(g gin.IRouter) {
	g.Use(r.serverFile)
}

func (r routerImpl) serverFile(c *gin.Context) {
	if 404 != c.Writer.Status() || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
		c.Next()
		return
	}

	if !strings.HasPrefix(c.Request.URL.Path, r.prefix) {
		c.Next()
		return
	}

	url := c.Request.URL.Path
	p := strings.TrimPrefix(c.Request.URL.Path, r.prefix[:len(r.prefix)-1])
	p = path.Clean(p)

	var indexPage bool
	if p == "/" {
		indexPage = true
		p = "/index.html"
	} else if url[len(url)-1] == '/' {
		indexPage = true
		p = p + "/index.html"
	}

	f, err := r.fs.Open(p)
	if err != nil {
		handleError(err, c)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		handleError(err, c)
		return
	}

	if stat.IsDir() {
		if !indexPage {
			c.Redirect(301, path.Base(url)+"/")
		} else {
			c.Next()
		}
		return
	}

	http.ServeContent(c.Writer, c.Request, stat.Name(), stat.ModTime(), f)
	c.Abort()
}

func handleError(err error, c *gin.Context) {
	if os.IsNotExist(err) {
		c.Next()
	} else if os.IsPermission(err) {
		c.AbortWithStatus(403)
	} else {
		c.AbortWithStatus(500)
	}
}
