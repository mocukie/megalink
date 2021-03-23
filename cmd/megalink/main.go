package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mocukie/megalink"
	"github.com/mocukie/megalink/web"
	"github.com/mocukie/megalink/web/dl"
	"github.com/mocukie/megalink/web/static"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
)

var version = "0.1.0"

func windowsBrokenPipeRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "an established connection was aborted by the software in your host machine") ||
							strings.Contains(strings.ToLower(se.Error()), "an existing connection was forcibly closed by the remote host") {
							brokenPipe = true
						}
					}
				}
				if brokenPipe {
					c.Error(err.(error))
					c.Abort()
				} else {
					panic(err) //rethrow
				}
			}
		}()
		c.Next()
	}
}

func setupRouter(e *gin.Engine) {
	f, _ := fs.Sub(megalink.WWW, "www")
	routers := []web.IRouter{
		dl.NewRouter(),
		static.NewRouter("/", http.FS(f)),
	}

	for _, r := range routers {
		r.Setup(e)
	}
}

func main() {
	const (
		OptionServerAddr = "addr"
		OptionTLSCert    = "tls.cert"
		OptionTLSKey     = "tls.key"
	)

	pflag.StringP(OptionServerAddr, "a", "127.0.0.1:30303", "server listen address")
	pflag.String(OptionTLSCert, "", "TLS certificate file path")
	pflag.String(OptionTLSKey, "", "TLS key file path")
	printVer := pflag.BoolP("version", "v", false, "print version")
	pflag.Parse()

	if *printVer {
		fmt.Printf("megalink %s (%s %s/%s)", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("MEGALINK")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	_ = viper.BindPFlags(pflag.CommandLine)

	addr := viper.GetString(OptionServerAddr)
	if viper.GetBool("debug") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()
	engine.Use(windowsBrokenPipeRecovery(), func(c *gin.Context) {
		c.Header("Server", "nginx/1.14.514")
		c.Next()
	})
	setupRouter(engine)

	var err error
	if cert, key := viper.GetString(OptionTLSCert), viper.GetString(OptionTLSKey); cert != "" && key != "" {
		fmt.Printf("Listening and serving HTTPS on %s\n", addr)
		err = http.ListenAndServeTLS(addr, cert, key, engine)
	} else {
		fmt.Printf("Listening and serving HTTP on %s\n", addr)
		err = http.ListenAndServe(addr, engine)
	}

	if err != nil {
		log.Fatalf("start server failed, casuse: %+v", err)
	}
}
