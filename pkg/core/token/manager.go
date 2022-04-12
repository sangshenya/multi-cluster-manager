package token

import (
	"context"
	"crypto/md5"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"harmonycloud.cn/stellaris/pkg/common/helper"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	publicKey  = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCHJLKxt2y3szY1k5O5yq3wbyPWRo57wl5Zyhl60oV/PCG0pEWBzDeab9eeMNein103TkNj7qFZLaqCLGb6fTz6sV9fqSZUlcl4F0twxo6LIoaKt925Wj1E93duYFbaM0uGJkqdU+OcRbvCODE/3M34wj/6TLKl/PXwWU76PTjp5wIDAQAB"
	privateKey = "MIICdQIBADANBgkqhkiG9w0BAQEFAASCAl8wggJbAgEAAoGBAIcksrG3bLezNjWTk7nKrfBvI9ZGjnvCXlnKGXrShX88IbSkRYHMN5pv154w16KfXTdOQ2PuoVktqoIsZvp9PPqxX1+pJlSVyXgXS3DGjosihoq33blaPUT3d25gVtozS4YmSp1T45xFu8I4MT/czfjCP/pMsqX89fBZTvo9OOnnAgMBAAECgYBp7XjnVbewkZcXDZLIGTaXc/XqGanLFcHwrTmljOe4oEBnIC+fGpwmwC2IwA31WOau1/h4lu3/QY0ZtYYOJyYo0N0mxDDljkI2mbf/p7DfUosDr3cYe0VakA3yllKjAqK8AtJ9DFDgoUv1dB3Pw4banTqNl3lBEVuVHadNnj9qMQJBAM2Ormaf/9bvhYCGxYeT8nug19RyLbfU9fyerFYjZHFxiUpppIFlgkohNbeQ0ZFb9gBq3Foi4CAcSL0FN5Rr+k8CQQCoTonz8NiBPGNtRGkQ5fqyPPbvtKT5U42IBzTZew/d//gYyk1siNTTQklS5XQfr8vO4Os+prbnbVMu2ObMHmjpAkB2g1Hn102xBU3KSWmvfkwqnRRy5xWWzJC6gl1IGIW7pkMKhRgUhor05GrNGBDLpuKRYQsEaOEhgk0ptc1SpGKfAkB7hiDjY1lTCGIkmLfPyipDVFEbvmXyAyt1sWxNTW9ozGtmrltCk+43Gog8CeE/PEOFkze0JKFKmscZM+G33325AkBk3uaOyjnP2lpNelN5iFeCbfdvDnzoM7I+/oZfpdyyjkJLzSpl7mAX3xmR2J0M+6nEaownhSk/MQTPi5bC1Inm"
)

func CreateToken(ctx context.Context, clientSet client.Client) error {
	token, err := sign(GetRandom())
	if err != nil {
		return err
	}
	if len(token) == 0 {
		return errors.New("token is empty")
	}
	// create token cm
	name, err := helper.GetOperatorName()
	if err != nil {
		name = "stellaris-core"
	}
	name = name + "-register-token"
	namespace, err := helper.GetOperatorNamespace()
	if err != nil {
		namespace = "stellaris-system"
	}

	tokenCm := &v1.ConfigMap{}
	tokenCm.SetNamespace(namespace)
	tokenCm.SetName(name)
	tokenCm.Data = map[string]string{
		"token": token,
	}
	err = clientSet.Create(ctx, tokenCm)
	return err
}

func sign(str string) (string, error) {
	md5Str := strings.ToUpper(str)

	data, err := RsaEncrypt([]byte(md5Str), publicKey)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func RsaEncrypt(origData []byte, publicKey string) ([]byte, error) {
	bytes_publickey, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return nil, err
	}
	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: bytes_publickey,
	}

	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub := pubInterface.(*rsa.PublicKey)
	return rsa.EncryptPKCS1v15(cryptorand.Reader, pub, origData)
}

func GetRandom() string {
	unixStr := strconv.FormatInt(time.Now().UnixNano(), 10)
	str := strconv.Itoa(rand.Intn(99))
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(unixStr + str))
	data := md5Ctx.Sum(nil)
	return hex.EncodeToString(data[:])
}
