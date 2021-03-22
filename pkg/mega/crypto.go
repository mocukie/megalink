package mega

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"github.com/joomcode/errorx"
	"unsafe"
)

var (
	zeroIV     [aes.BlockSize]byte
	aesSkipBuf [aes.BlockSize]byte
	b64        = base64.URLEncoding.WithPadding(base64.NoPadding)
)

type ecbDecrypter struct {
	b cipher.Block
}

func newECBDecrypter(block cipher.Block) cipher.BlockMode {
	return &ecbDecrypter{b: block}
}

func (e *ecbDecrypter) BlockSize() int {
	return e.b.BlockSize()
}

func (e *ecbDecrypter) CryptBlocks(dst, src []byte) {
	blkSize := e.b.BlockSize()
	if len(src)%blkSize != 0 {
		panic("ecb decrypter: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("ecb decrypter: output smaller than input")
	}
	if len(src) == 0 {
		return
	}

	for i := 0; i < len(src); i += blkSize {
		e.b.Decrypt(dst[i:], src[i:])
	}
}

func NewAesCTRStream(blk cipher.Block, iv []byte, off uint64) (ctr cipher.Stream) {
	if off == 0 {
		return cipher.NewCTR(blk, iv)
	}
	iv = CalcAesCTRIV(iv, off)
	ctr = cipher.NewCTR(blk, iv)
	if skip := off % aes.BlockSize; skip > 0 {
		ctr.XORKeyStream(aesSkipBuf[:skip], aesSkipBuf[:skip])
	}
	return
}

func CalcAesCTRIV(iv []byte, off uint64) []byte {
	if len(iv) != aes.BlockSize {
		panic("mega.CalcAesCTRIV: IV length must equal block size")
	}

	hi := binary.BigEndian.Uint64(iv)
	lo := binary.BigEndian.Uint64(iv[8:])
	for blocks := off / aes.BlockSize; blocks > 0; blocks-- {
		lo++
		if lo == 0 {
			hi++
		}
	}
	iv = make([]byte, aes.BlockSize)
	binary.BigEndian.PutUint64(iv, hi)
	binary.BigEndian.PutUint64(iv[8:], lo)
	return iv
}

func unpackKeyB64(key string) (aesKey, iv, mac []byte, err error) {
	k, err := b64.DecodeString(key)
	if err != nil {
		err = errorx.Decorate(err, "decode base64 failed")
		return
	}
	aesKey, iv, mac = unpackKey(k)
	return
}

func unpackKey(key []byte) (aesKey, iv, mac []byte) {
	var b [40]byte
	aesKey, iv, mac = b[:16], b[16:32], b[32:]
	*(*uint64)(unsafe.Pointer(&aesKey[0])) = *(*uint64)(unsafe.Pointer(&key[0])) ^ *(*uint64)(unsafe.Pointer(&key[16]))
	*(*uint64)(unsafe.Pointer(&aesKey[8])) = *(*uint64)(unsafe.Pointer(&key[8])) ^ *(*uint64)(unsafe.Pointer(&key[24]))
	copy(iv, key[16:24])
	copy(mac, key[24:32])
	return
}

func decryptNodeKey(key []byte, masterKey cipher.Block) []byte {
	dst := make([]byte, len(key))
	newECBDecrypter(masterKey).CryptBlocks(dst, key)
	return dst
}

func decryptAttr(dst *Attribute, src string, aesKey []byte) error {
	attr, err := b64.DecodeString(src)
	if err != nil {
		return errorx.Decorate(err, "decode base64 failed")
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return errorx.Decorate(err, "invalid aes key")
	}

	buf := make([]byte, len(attr))
	cbc := cipher.NewCBCDecrypter(block, zeroIV[:])
	cbc.CryptBlocks(buf, attr)

	if !bytes.HasPrefix(buf, []byte("MEGA")) {
		return errorx.Decorate(ErrDecryptAttr, "invalid magic: "+hex.EncodeToString(buf[:4]))
	}
	buf = buf[4:]
	buf = bytes.Trim(buf, "\x00")

	err = json.Unmarshal(buf, dst)
	if err != nil {
		return errorx.Decorate(err, "decode json failed")
	}
	return nil
}
