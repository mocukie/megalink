package mega

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"math/rand"
	"time"

	"testing"
)

func TestNewAesCTRStream(t *testing.T) {
	var plainText = []byte(`Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.`)
	var cipherText = make([]byte, len(plainText))
	var key = []byte("!QAZ2wsx1qaz@WSX")
	var iv = make([]byte, aes.BlockSize)

	rand.Seed(time.Now().UnixNano())
	_, err := rand.Read(iv[:8])
	if err != nil {
		t.Fatal(err)
	}

	blk, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	// encrypt
	cipher.NewCTR(blk, iv).XORKeyStream(cipherText, plainText)

	// decrypt random offset
	off := rand.Intn(len(plainText))
	ctr := NewAesCTRStream(blk, iv, uint64(off))
	tmp := make([]byte, len(plainText)-off)
	ctr.XORKeyStream(tmp, cipherText[off:])

	t.Log("off", off)
	t.Log("plain    : ", string(plainText[off:]))
	t.Log("decrypted: ", string(tmp))

	if !bytes.Equal(tmp, plainText[off:]) {
		t.Fail()
	}
}
