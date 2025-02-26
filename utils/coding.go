package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
)

func getHash(algorithm string, s ...interface{}) (r string) {
	r = hex.EncodeToString(hashBytes(algorithm, s...))
	return
}

func hashBytes(algorithm string, s ...interface{}) (r []byte) {
	var h hash.Hash
	switch algorithm {
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha2", "sha256":
		h = sha256.New()
	}
	for _, value := range s {
		switch value.(type) {
		case []byte:
			h.Write(value.([]byte))
		default:
			h.Write([]byte(ToString(value)))
		}
	}
	r = h.Sum(nil)

	return
}

// Md5 MD5加密
func Md5(s ...interface{}) (r string) {
	return getHash("md5", s...)
}

// JSONDecode Json格式解析
func JSONDecode(str string) (p P) {
	_bytes := []byte(str)
	if len(_bytes) <= 0 {
		return p
	}
	err := json.Unmarshal(_bytes, &p)
	if err != nil {
		Error(fmt.Sprintf("JSONDecode Error:%v,Str:%v", err.Error(), str))
	}

	return
}

// JSONEncode Json格式编码
func JSONEncode(value interface{}) string {
	_bytes, err := json.Marshal(value)
	if err != nil {
		Error("JSONEncode Error:", value, err)
		return ""
	}

	return string(_bytes)
}

// Base64Encode Base64格式编码
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode Base64格式解码
func Base64Decode(src string) []byte {
	r, e := base64.StdEncoding.DecodeString(src)
	if e != nil {
		Error(e)
	}

	return r
}
