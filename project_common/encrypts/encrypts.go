package encrypts

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/hex"
	"io"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

// Md5 仅用于缓存 key 生成等非安全场景，不用于密码存储
func Md5(str string) string {
	hash := md5.New()
	_, _ = io.WriteString(hash, str)
	return hex.EncodeToString(hash.Sum(nil))
}

// HashPassword 使用 bcrypt 对密码进行哈希，用于注册时存储密码
// bcrypt 内置随机盐 + 自适应 cost，无需额外管理盐值
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// VerifyPassword 验证明文密码是否与 bcrypt 哈希匹配，用于登录校验
// 返回 true 表示密码正确
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

var commonIV = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

func EncryptInt64(id int64, keyText string) (cipherStr string, err error) {
	idStr := strconv.FormatInt(id, 10)
	return Encrypt(idStr, keyText)
}

var AESKey = "sdfgyrhgbxcdgryfhgywertd"

func DecryptNoErr(cipherStr string) int64 {
	decrypt, _ := Decrypt(cipherStr, AESKey)
	parseInt, _ := strconv.ParseInt(decrypt, 10, 64)
	return parseInt
}

func EncryptNoErr(id int64) string {
	str, _ := EncryptInt64(id, AESKey)
	return str
}

func Encrypt(plainText string, keyText string) (cipherStr string, err error) {
	// 转换成字节数据, 方便加密
	plainByte := []byte(plainText)
	keyByte := []byte(keyText)
	// 创建加密算法aes
	c, err := aes.NewCipher(keyByte)
	if err != nil {
		return "", err
	}
	//使用CFB模式和固定的初始向量（保证即使明文和密钥相同，每次加密结果也不同）创建加密流。
	cfb := cipher.NewCFBEncrypter(c, commonIV)
	cipherByte := make([]byte, len(plainByte))
	// 使用XORKeyStream方法对明文进行加密
	cfb.XORKeyStream(cipherByte, plainByte)
	cipherStr = hex.EncodeToString(cipherByte)
	return
}

func Decrypt(cipherStr string, keyText string) (plainText string, err error) {
	// 转换成字节数据, 方便加密
	keyByte := []byte(keyText)
	// 创建加密算法aes
	c, err := aes.NewCipher(keyByte)
	if err != nil {
		return "", err
	}
	// 解密字符串
	cfbdec := cipher.NewCFBDecrypter(c, commonIV)
	cipherByte, _ := hex.DecodeString(cipherStr)
	plainByte := make([]byte, len(cipherByte))
	cfbdec.XORKeyStream(plainByte, cipherByte)
	plainText = string(plainByte)
	return
}
