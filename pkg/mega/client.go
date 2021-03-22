package mega

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"github.com/joomcode/errorx"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
)

const (
	ApiURL              = "https://g.api.mega.co.nz/cs"
	HandleLen           = 8
	FolderNodeKeyB64Len = 22
	FileNodeKeyB64Len   = 43
)

type Client struct {
	seq        uint64
	httpClient *http.Client
}

func NewClient(client *http.Client) *Client {
	return &Client{
		httpClient: client,
	}
}

func (c *Client) apiSend(query url.Values, request interface{}, response interface{}) error {
	body, err := json.Marshal([]interface{}{request})
	if err != nil {

		return errorx.Decorate(err, "encode json failed")
	}

	var seq = atomic.AddUint64(&c.seq, 1)
	var reqUrl string
	if query != nil && len(query) != 0 {
		reqUrl = fmt.Sprintf("%s?id=%d&%s", ApiURL, seq, query.Encode())
	} else {
		reqUrl = fmt.Sprintf("%s?id=%d", ApiURL, seq)
	}

	resp, err := http.Post(reqUrl, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return errorx.Decorate(err, "http post net error")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return errorx.Decorate(HttpStatusErr(resp.StatusCode), "invalid http status")
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errorx.Decorate(err, "failed to read response")
	}

	data = bytes.TrimSpace(data)
	//remove json array [ and ]
	if len(data) >= 2 && data[0] == '[' && data[len(data)-1] == ']' {
		data = data[1 : len(data)-1]
	}

	err = json.Unmarshal(data, response)
	if err != nil {
		var code int
		if e := json.Unmarshal(data, &code); e == nil {
			err = errorx.Decorate(ApiErr(code), "")
		} else {
			err = errorx.Decorate(err, "decode json failed")
		}
		return err
	}

	return nil
}

// from official web client mega.js treefetcher_fetch
type NodesReq struct {
	A  string `json:"a"`
	C  int    `json:"c"`
	R  int    `json:"r"`  //recursive?
	CA int    `json:"ca"` //cache?
}

type NodesResp struct {
	F  []EncryptedNode `json:"f"`
	SN string          `json:"sn"`
}

type EncryptedNode struct {
	H  string   `json:"h"`
	P  string   `json:"p"`
	U  string   `json:"u"`
	K  string   `json:"k"`
	Ts int64    `json:"ts"`
	S  int64    `json:"s"`
	A  string   `json:"a"`
	T  NodeType `json:"t"`
}

func (c *Client) OpenPublicFolder(handle, key string) (fm *FM, err error) {
	if len(key) != FolderNodeKeyB64Len {
		return nil, ErrInvalidKeyLen
	}

	aesKey, err := b64.DecodeString(key)
	if err != nil {
		return
	}

	blk, err := aes.NewCipher(aesKey)
	if err != nil {
		return
	}

	var resp NodesResp
	var req = NodesReq{
		A: "f",
		C: 1,
		R: 1,
	}
	if err = c.apiSend(url.Values{"n": []string{handle}}, &req, &resp); err != nil {
		return
	}

	fm = &FM{
		client:    c,
		handle:    handle,
		masterKey: blk,
		root:      &Node{Type: TypeFolder},
		lookup:    make(map[string]*Node, len(resp.F)),
	}
	for i := range resp.F {
		if err = fm.addNode(&resp.F[i]); err != nil {
			return
		}
	}
	return
}

type NodeInfoReq struct {
	A   string `json:"a"`
	G   int    `json:"g"`
	SSL int    `json:"ssl"`         // use https
	P   string `json:"p,omitempty"` // public handle
	N   string `json:"n,omitempty"` // node handle
}

type NodeInfoResp struct {
	S   int64  `json:"s"`
	At  string `json:"at"`
	URL string `json:"g"`
}

func (c *Client) getFileNodeInfo(ph, handle string, key *NodeKey) (info *NodeInfo, err error) {
	var query = url.Values{}
	var resp NodeInfoResp
	var req = NodeInfoReq{
		A:   "g",
		G:   1,
		SSL: 1,
	}
	if ph == handle {
		req.P = handle
	} else {
		req.N = handle
		query.Set("n", ph)
	}

	if err = c.apiSend(query, &req, &resp); err != nil {
		return
	}

	info = &NodeInfo{
		Size: resp.S,
		URL:  resp.URL,
		K:    *key,
	}
	err = decryptAttr(&info.Attr, resp.At, key.Key)
	return
}

func (c *Client) GetPublicFileNodeInfo(publicHandle, nodeHandle string, nodeKey string) (info *NodeInfo, err error) {
	if len(nodeKey) != FileNodeKeyB64Len {
		return nil, ErrInvalidKeyLen
	}

	var k NodeKey
	k.Key, k.IV, k.Mac, err = unpackKeyB64(nodeKey)
	if err != nil {
		err = errorx.Decorate(err, "unpack key failed")
		return
	}

	info, err = c.getFileNodeInfo(publicHandle, nodeHandle, &k)
	return
}

type Download struct {
	data  io.ReadCloser
	ctr   cipher.Stream
	Range struct {
		S     int64
		E     int64
		Total int64
	}
	Http struct {
		Status     string
		StatusCode int
		Header     http.Header
	}
}

func (d *Download) Read(p []byte) (n int, err error) {
	n, err = d.data.Read(p)
	d.ctr.XORKeyStream(p[:n], p[:n])
	return
}

func (d *Download) Close() error {
	return d.data.Close()
}

type DownloadOption func(*http.Request) error

// http range
func (d DownloadOption) Range(s, e int64) DownloadOption {
	return func(req *http.Request) error {
		if err := d(req); err != nil {
			return err
		}
		if s < 0 || (e < s && e != -1) {
			return HttpStatusErr(http.StatusRequestedRangeNotSatisfiable)
		}

		var r string
		if e == -1 {
			r = fmt.Sprintf("bytes=%d-", s)
		} else {
			r = fmt.Sprintf("bytes=%d-%d", s, e)
		}
		req.Header.Set("Range", r)
		return nil
	}
}

func (d DownloadOption) HttpHeader(k, v string) DownloadOption {
	return func(req *http.Request) error {
		if err := d(req); err != nil {
			return err
		}
		if strings.ToLower(k) == "range" {
			return nil
		}
		req.Header.Set(k, v)
		return nil
	}
}

func NewDownloadOption() DownloadOption {
	return func(req *http.Request) error {
		return nil
	}
}

var rangeRegex = regexp.MustCompile("^bytes (\\d+)-(\\d+)/(\\d+)$")

func (c *Client) Download(info *NodeInfo, opt DownloadOption) (dl *Download, err error) {

	blk, err := aes.NewCipher(info.K.Key)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodGet, info.URL, nil)
	if err != nil {
		return
	}

	if opt != nil {
		err = opt(req)
		if err != nil {
			err = errorx.Decorate(err, "invalid download option")
			return
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}

	dl = new(Download)
	dl.Http.Status, dl.Http.StatusCode, dl.Http.Header = resp.Status, resp.StatusCode, resp.Header
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		err = errorx.Decorate(HttpStatusErr(resp.StatusCode), "invalid http status")
		return
	}

	s, e, t := 0, int(info.Size-1), int(info.Size)
	if resp.StatusCode == http.StatusPartialContent {
		rg := resp.Header.Get("Content-Range")
		if g := rangeRegex.FindStringSubmatch(rg); len(g) > 0 {
			s, _ = strconv.Atoi(g[1])
			e, _ = strconv.Atoi(g[2])
			t, _ = strconv.Atoi(g[3])
		}
	}

	dl.Range.S, dl.Range.E, dl.Range.Total = int64(s), int64(e), int64(t)
	dl.ctr = NewAesCTRStream(blk, info.K.IV, uint64(s))
	dl.data = resp.Body
	return
}
