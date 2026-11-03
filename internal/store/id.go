package store

import (
	"crypto/rand"
	"strings"
	"time"
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewID returns a ULID: 26 Crockford base32 characters, a 48-bit
// millisecond timestamp then 80 random bits. IDs sort by creation
// millisecond; IDs created in the same millisecond sort randomly.
func NewID(now time.Time) string {
	var b [16]byte
	ms := uint64(now.UnixMilli())
	for i := range 6 {
		b[i] = byte(ms >> (40 - 8*i))
	}
	rand.Read(b[6:])

	var id [26]byte
	var acc uint32
	bits, pos := 2, 0
	for _, c := range b {
		acc = acc<<8 | uint32(c)
		bits += 8
		for bits >= 5 {
			bits -= 5
			id[pos] = crockford[(acc>>bits)&31]
			pos++
		}
	}
	return string(id[:])
}

func validID(id string) bool {
	if len(id) != 26 {
		return false
	}
	for _, r := range id {
		if !strings.ContainsRune(crockford, r) {
			return false
		}
	}
	return true
}
