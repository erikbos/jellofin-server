package idhash

import (
	"crypto/rand"
	"crypto/sha256"
	"math/big"

	"github.com/jxskiss/base62"
)

// Hash a string with sha256.
// Then take the first 119 bits, and convert that to base62.
// Returns a 20-character long string.
func IdHash(name string) string {
	// Create hash from string.
	hash256 := sha256.Sum256([]byte(name))

	// Create 128 bit integer from the first 8 bytes of the hash.
	num128 := big.NewInt(0)
	num128.SetBytes(hash256[:16])

	// Use only the first 119 bits.
	num128.Rsh(num128, 9)

	const62 := big.NewInt(62)
	mod := big.NewInt(0)

	// into base62.
	id := ""
	for i := 0; i < 20; i++ {
		mod.Mod(num128, const62)
		m := int(mod.Int64())
		num128.Div(num128, const62)

		c := 33
		if m < 10 {
			c = m + 48
		} else if m < 36 {
			c = m + 65 - 10
		} else if m < 62 {
			c = m + 97 - 36
		}
		id += string(c)
	}

	return id
}

// Hash returns a base62-encoded id, based upon sha256 of string.
func Hash(s string) string {
	return HashBytes([]byte(s))
}

// HashBytes returns a base62-encoded id, based upon sha256 of bytes.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return base62.StdEncoding.EncodeToString(sum[:16])
}

// NewRandomID generates a random base62-encoded id.
func NewRandomID() string {
	var r [16]byte
	if _, err := rand.Read(r[:]); err != nil {
		panic(err)
	}
	return base62.StdEncoding.EncodeToString(r[:])
}
