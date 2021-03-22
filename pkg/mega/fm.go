package mega

import (
	"crypto/cipher"
	"github.com/joomcode/errorx"
	"path"
	"strings"
)

type NodeType int32

const (
	TypeFile NodeType = iota
	TypeFolder
	TypeCloudDrive
	TypeInbox
	TypeTrashBin
)

type NodeKey struct {
	Key []byte
	IV  []byte
	Mac []byte
}

type Attribute struct {
	Name string `json:"n"`
}

type NodeInfo struct {
	Size int64
	Attr Attribute
	K    NodeKey
	URL  string
}

type Node struct {
	Handle    string
	Owner     string
	Parent    string
	Children  []*Node
	Type      NodeType
	Timestamp int64
	Size      int64
	Attr      Attribute
	K         NodeKey
}

func (n *Node) Walk(walker func(*Node) bool) bool {
	for _, child := range n.Children {
		if !walker(child) {
			return false
		}
		if len(child.Children) == 0 {
			continue
		}
		if !child.Walk(walker) {
			return false
		}
	}
	return true
}

type FM struct {
	client    *Client
	handle    string
	masterKey cipher.Block
	root      *Node
	lookup    map[string]*Node
}

func (fm *FM) addNode(en *EncryptedNode) error {

	idx := strings.Index(en.K, ":")
	if idx == -1 {
		return nil
	}

	encryptedKey, err := b64.DecodeString(en.K[idx+1:])
	if err != nil {
		return errorx.Decorate(err, "decode base64 failed: ")
	}

	n, ok := fm.lookup[en.H]
	if !ok {
		n = new(Node)
	}
	n.Handle = en.H
	n.Parent = en.P
	n.Owner = en.U
	n.Type = en.T
	n.Size = en.S
	n.Timestamp = en.Ts

	key := decryptNodeKey(encryptedKey, fm.masterKey)
	switch n.Type {
	case TypeFile:
		n.K.Key, n.K.IV, n.K.Mac = unpackKey(key)
	case TypeFolder:
		n.K.Key = key
	}
	if en.A != "" {
		err = decryptAttr(&n.Attr, en.A, n.K.Key)
		if err != nil {
			return err
		}
	}

	if en.P != "" && len(fm.root.Children) != 0 { // first node is public root
		parent, ok := fm.lookup[en.P]
		if !ok {
			parent = &Node{
				Handle: en.P,
				Type:   TypeFolder,
			}
			fm.lookup[en.P] = parent
		}
		parent.Children = append(parent.Children, n)
	} else {
		fm.root.Children = append(fm.root.Children, n)
	}

	fm.lookup[en.H] = n
	return nil
}

func (fm *FM) Lookup(handle string) *Node {
	return fm.lookup[handle]
}

func (fm *FM) LookupPath(p string) *Node {
	if strings.HasPrefix(p, "/") {
		p = p[1:]
	}

	p = path.Clean(p)
	if p == "." {
		return fm.root
	}

	var n = fm.root
	var i, ok = 0, false
	for _, c := range p {
		if c != '/' {
			i++
			continue
		}
		if n, ok = fm.lookup[p[:i]]; !ok {
			return nil
		}
		p = p[i+1:]
		i = 0
	}
	if p != "" {
		n = fm.lookup[p]
	}
	return n
}

func (fm *FM) Walk(walker func(*Node) bool) {
	fm.root.Walk(walker)
}

func (fm *FM) GetFileNodeInfo(n *Node) (info *NodeInfo, err error) {
	if n.Type != TypeFile {
		return nil, errorx.Decorate(ErrInvalidNodeType, "")
	}

	n, ok := fm.lookup[n.Handle]
	if !ok {
		return nil, errorx.Decorate(API_ENOENT, "")
	}

	info, err = fm.client.getFileNodeInfo(fm.handle, n.Handle, &n.K)
	return
}
