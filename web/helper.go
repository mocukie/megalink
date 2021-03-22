package web

import (
	"crypto/aes"
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mocukie/megalink/pkg/errutil"
	"github.com/mocukie/megalink/pkg/mega"
	"net/http"
	"regexp"
)

var (
	FileLinkRegexs = []*regexp.Regexp{
		regexp.MustCompile(`^!!([a-zA-Z\d_-]{8})!([a-zA-Z\d_-]{43})$`),
		regexp.MustCompile(`^([a-zA-Z\d_-]{8})!([a-zA-Z\d_-]{43})$`),
	}
	FolderLinkRegex = regexp.MustCompile(`^([a-zA-Z\d_-]{8})!([a-zA-Z\d_-]{22})$`)
	MegaClient      = mega.NewClient(http.DefaultClient)
)

type IRouter interface {
	Setup(group gin.IRouter)
}

type errDetail struct {
	Err error
}

func (e errDetail) Format(f fmt.State, verb rune) {
	if verb == 'v' {
		fmt.Fprintf(f, "%+v", e.Err)
	} else {
		fmt.Fprintf(f, "%v", e.Err)
	}
}

func ConvertError(err error) (parsedErr *gin.Error, code int, msg string) {
	code = 500
	typ := gin.ErrorTypePrivate
	cause := errutil.Cause(err)
	switch e := cause.(type) {
	case mega.HttpStatusErr:
		code = int(e)
		msg = "MEGA api invalid status"
	case mega.ApiErr:
		switch e {
		case mega.API_EINTERNAL:
			code = 500
		case mega.API_EARGS:
			code = 400
		case mega.API_EAGAIN:
			code = 503
		case mega.API_ERATELIMIT, mega.API_ETOOMANY, mega.API_ETOOMANYCONNECTIONS:
			code = 429
		case mega.API_ENOENT, mega.API_ETEMPUNAVAIL:
			code = 404
		case mega.API_EACCESS:
			code = 403
		case mega.API_EBLOCKED:
			code = 451
		default:
			code = 400
		}
		typ = gin.ErrorTypePublic
		msg = e.Error()
	case base64.CorruptInputError, aes.KeySizeError:
		code = 400
	default:
		switch cause {
		case mega.ErrDecryptAttr, mega.ErrInvalidKeyLen:
			code = 400
			typ = gin.ErrorTypePublic
			msg = mega.API_EKEY.Message()
		}
	}

	parsedErr = &gin.Error{
		Err:  err,
		Type: typ,
		Meta: errDetail{Err: err},
	}
	return
}
