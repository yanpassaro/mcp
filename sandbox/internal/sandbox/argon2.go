package sandbox

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	lua "github.com/Shopify/go-lua"
	"golang.org/x/crypto/argon2"
)

const (
	argon2DefaultSaltLen  uint32 = 16
	argon2DefaultKeyLen   uint32 = 32
	argon2DefaultMemory   uint32 = 64 * 1024
	argon2DefaultIters    uint32 = 3
	argon2DefaultParallel uint8  = 2
	argon2MaxMemory       uint32 = 1 << 20
)

func argon2HashFn(l *lua.State) int {
	password := argString(l, 1)
	if password == "" {
		panic(errors.New("argon2: password cannot be empty"))
	}

	saltLen := argon2DefaultSaltLen
	keyLen := argon2DefaultKeyLen
	memory := argon2DefaultMemory
	iterations := argon2DefaultIters
	parallel := argon2DefaultParallel
	var salt []byte

	if opts := toAnyMap(l, 2); len(opts) > 0 {
		memory = optUint(opts, "memory", memory)
		iterations = optUint(opts, "iterations", iterations)
		saltLen = optUint(opts, "salt_length", saltLen)
		keyLen = optUint(opts, "key_length", keyLen)
		parallel = optUint8(opts, "parallelism", parallel)
		if s := optString(opts, "salt"); s != "" {
			b, err := phcB64Decode(s)
			if err != nil {
				panic(fmt.Errorf("argon2: salt must be base64: %v", err))
			}
			salt = b
		}
	}
	if salt == nil {
		salt = randomBytes(int(saltLen))
	}

	validateArgonParams(memory, iterations, parallel)

	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallel, keyLen)

	phc := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, iterations, parallel,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	l.PushString(phc)
	return 1
}

func argon2VerifyFn(l *lua.State) int {
	password := argString(l, 1)
	phc := strings.TrimSpace(argString(l, 2))
	if phc == "" {
		panic(errors.New("argon2_verify: hash cannot be empty"))
	}
	ok, err := verifyArgon2(password, phc)
	if err != nil {
		panic(err)
	}
	l.PushBoolean(ok)
	return 1
}

func verifyArgon2(password, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("argon2_verify: invalid argon2id PHC hash")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("argon2_verify: invalid version: %v", err)
	}
	if version != argon2.Version {
		return false, fmt.Errorf("argon2_verify: unsupported version %d", version)
	}

	var memory, iterations uint32
	var parallel uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallel); err != nil {
		return false, fmt.Errorf("argon2_verify: invalid params: %v", err)
	}
	validateArgonParams(memory, iterations, parallel)

	salt, err := phcB64Decode(parts[4])
	if err != nil {
		return false, fmt.Errorf("argon2_verify: invalid salt: %v", err)
	}
	want, err := phcB64Decode(parts[5])
	if err != nil {
		return false, fmt.Errorf("argon2_verify: invalid hash: %v", err)
	}

	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallel, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func validateArgonParams(memory, iterations uint32, parallel uint8) {
	if memory < 8*1024 {
		panic(fmt.Errorf("argon2: memory must be at least 8 KiB"))
	}
	if memory > argon2MaxMemory {
		panic(fmt.Errorf("argon2: memory too large (max %d KiB)", argon2MaxMemory))
	}
	if iterations < 1 || iterations > 64 {
		panic(fmt.Errorf("argon2: iterations must be 1..64"))
	}
	if parallel < 1 || parallel > 32 {
		panic(fmt.Errorf("argon2: parallelism must be 1..32"))
	}
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("argon2: failed to read random bytes: %v", err))
	}
	return b
}

func phcB64Decode(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.RawStdEncoding,
		base64.StdEncoding,
		base64.RawURLEncoding,
		base64.URLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("invalid base64")
}

func optUint(m map[string]any, key string, def uint32) uint32 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return uint32(n)
		case int:
			return uint32(n)
		case int64:
			return uint32(n)
		case uint32:
			return n
		}
	}
	return def
}

func optUint8(m map[string]any, key string, def uint8) uint8 {
	return uint8(optUint(m, key, uint32(def)))
}

func optString(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

func asBool(v any, def bool) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return strings.EqualFold(strings.TrimSpace(b), "true")
	case float64:
		return b != 0
	case int:
		return b != 0
	case int64:
		return b != 0
	}
	return def
}
