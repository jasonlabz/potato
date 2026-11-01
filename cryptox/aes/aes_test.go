package aes

import (
	"bytes"
	"testing"
)

func TestRoundTripAndBadInput(t *testing.T) {
	for _, keyLen := range []int{16, 24, 32} {
		c := NewAESCrypto(bytes.Repeat([]byte("k"), keyLen))
		plain := []byte("\x00hello")
		for i := 0; i < 2; i++ { // 第二次结果必须与第一次一致（BlockMode 不跨调用链接）
			enc, err := c.Encrypt(plain)
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.Decrypt(enc)
			if err != nil || !bytes.Equal(got, plain) {
				t.Fatalf("key %d round %d: got %q, %v", keyLen, i, got, err)
			}
		}
		if _, err := c.Decrypt("AAAA"); err == nil {
			t.Fatalf("key %d: Decrypt short input should fail", keyLen)
		}
		if _, err := c.Decrypt(""); err == nil {
			t.Fatalf("key %d: Decrypt empty input should fail", keyLen)
		}
	}
}
