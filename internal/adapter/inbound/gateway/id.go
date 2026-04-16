package gateway

import (
	"crypto/rand"
	"fmt"
)

func newID(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%x", prefix, b)
}
