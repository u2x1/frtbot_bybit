package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"math"
	"net/url"
	"time"
)

func Truncate(num float64, minQty float64) float64 {
	return math.Trunc(num/minQty) * minQty
}

// FormatFloat formats a float number with the specified precision
func FormatFloat(num float64, precision int) string {
	formatString := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(formatString, num)
}

// AlignFloat aligns a float number to the nearest tick size with specified precision
func AlignFloat(num float64, tickSize float64, precision int) string {
	return FormatFloat(math.Round(num/tickSize)*tickSize, precision)
}

// EncodeMap encodes a map[string]string to a URL-encoded string
func EncodeMap(params map[string]string) string {
	values := url.Values{}
	for key, value := range params {
		values.Add(key, value)
	}
	return values.Encode()
}

// GenerateSignature generates HMAC SHA256 signature for Binance API
func GenerateSignature(params map[string]string, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(EncodeMap(params)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func GenerateSignatureString(message string, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(message))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Ticker creates a ticker that ticks at specified intervals until stopTime
func Ticker(offset time.Duration, interval time.Duration, stopTime time.Time) {
	now := time.Now()
	time.Sleep(now.Truncate(interval).Add(interval + offset).Sub(now))
	stopTime = stopTime.Add(offset)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		now = <-t.C
		if now.After(stopTime) {
			break
		}
	}
}
