package encrypts

import (
	"crypto/md5"
	"encoding/hex"
	"io"
)

func Md5(str string) string {
	hash := md5.New()
	_, _ = io.WriteString(hash, str)
	//将哈希值转换为十六进制字符串
	return hex.EncodeToString(hash.Sum(nil))
}
