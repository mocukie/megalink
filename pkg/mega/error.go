package mega

import (
	"errors"
	"fmt"
	"net/http"
)

type ApiErr int

func (e ApiErr) Error() string {
	return fmt.Sprintf("mega api code %d, %s", -e, apiErrorMsgs[e])
}

func (e ApiErr) Message() string {
	return apiErrorMsgs[e]
}

// from MEGA office sdk MError.h
const (
	API_OK                  ApiErr = 0
	API_EINTERNAL           ApiErr = -1  // internal error
	API_EARGS               ApiErr = -2  // bad arguments
	API_EAGAIN              ApiErr = -3  // request failed retry with exponential backoff
	API_ERATELIMIT          ApiErr = -4  // too many requests slow down
	API_EFAILED             ApiErr = -5  // request failed permanently
	API_ETOOMANY            ApiErr = -6  // too many requests for this resource
	API_ERANGE              ApiErr = -7  // resource access out of rage
	API_EEXPIRED            ApiErr = -8  // resource expired
	API_ENOENT              ApiErr = -9  // resource does not exist
	API_ECIRCULAR           ApiErr = -10 // circular linkage
	API_EACCESS             ApiErr = -11 // access denied
	API_EEXIST              ApiErr = -12 // resource already exists
	API_EINCOMPLETE         ApiErr = -13 // request incomplete
	API_EKEY                ApiErr = -14 // cryptographic error
	API_ESID                ApiErr = -15 // bad session ID
	API_EBLOCKED            ApiErr = -16 // resource administratively blocked
	API_EOVERQUOTA          ApiErr = -17 // quote exceeded
	API_ETEMPUNAVAIL        ApiErr = -18 // resource temporarily not available
	API_ETOOMANYCONNECTIONS ApiErr = -19 // too many connections on this resource
	API_EWRITE              ApiErr = -20 // file could not be written to
	API_EREAD               ApiErr = -21 // file could not be read from
	API_EAPPKEY             ApiErr = -22 // invalid or missing application key
	API_ESSL                ApiErr = -23 // SSL verification failed
	API_EGOINGOVERQUOTA     ApiErr = -24 // Not enough quota
	API_EMFAREQUIRED        ApiErr = -26 // Multi-factor authentication required
)

var apiErrorMsgs = map[ApiErr]string{
	API_OK:                  "",
	API_EINTERNAL:           "internal error",
	API_EARGS:               "bad arguments",
	API_EAGAIN:              "request failed retry with exponential backoff",
	API_ERATELIMIT:          "too many requests slow down",
	API_EFAILED:             "request failed permanently",
	API_ETOOMANY:            "too many requests for this resource",
	API_ERANGE:              "resource access out of rage",
	API_EEXPIRED:            "resource expired",
	API_ENOENT:              "resource does not exist",
	API_ECIRCULAR:           "circular linkage",
	API_EACCESS:             "access denied",
	API_EEXIST:              "resource already exists",
	API_EINCOMPLETE:         "request incomplete",
	API_EKEY:                "cryptographic error",
	API_ESID:                "bad session ID",
	API_EBLOCKED:            "resource administratively blocked",
	API_EOVERQUOTA:          "quote exceeded",
	API_ETEMPUNAVAIL:        "resource temporarily not available",
	API_ETOOMANYCONNECTIONS: "too many connections on this resource",
	API_EWRITE:              "file could not be written to",
	API_EREAD:               "file could not be read from",
	API_EAPPKEY:             "invalid or missing application key",
	API_ESSL:                "SSL verification failed",
	API_EGOINGOVERQUOTA:     "Not enough quota",
	API_EMFAREQUIRED:        "Multi-factor authentication required",
}

var ErrInvalidKeyLen = errors.New("invalid mega key")
var ErrInvalidNodeType = errors.New("invalid mega node type")
var ErrDecryptAttr = errors.New("decrypt mega attribute failed")

type HttpStatusErr int

func (e HttpStatusErr) Error() string {
	return fmt.Sprintf("%d %s", e, http.StatusText(int(e)))
}
