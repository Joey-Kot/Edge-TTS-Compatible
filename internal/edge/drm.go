package edge

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

const (
	trustedClientToken = "6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	windowsEpoch       = 11644473600
)

var clockSkewNano atomic.Int64

func secMSGEC(now time.Time) string {
	timestamp := now.UTC().Add(time.Duration(clockSkewNano.Load())).Unix()
	ticks := timestamp + windowsEpoch
	ticks -= ticks % 300
	fileTimeTicks := ticks * 10_000_000
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d%s", fileTimeTicks, trustedClientToken)))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func adjustClockSkew(serverDate time.Time, now time.Time) {
	clockSkewNano.Add(serverDate.Sub(now.UTC()).Nanoseconds())
}

func muid() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strings.ToUpper(hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano()))))
	}
	return strings.ToUpper(hex.EncodeToString(b[:]))
}
