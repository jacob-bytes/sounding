package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// signDingTalk 钉钉加签（HMAC-SHA256 + URL 编码）。
func signDingTalk(webhook, secret string) string {
	ts := fmt.Sprint(time.Now().UnixMilli())
	str := ts + "\n" + secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(str))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	sep := "?"
	if strings.Contains(webhook, "?") {
		sep = "&"
	}
	return webhook + sep + "timestamp=" + ts + "&sign=" + url.QueryEscape(sign)
}
