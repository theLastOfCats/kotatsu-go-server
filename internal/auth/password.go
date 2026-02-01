package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2 parameters
const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
)

// HashPassword hashes a password using Argon2id
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=65536,t=1,p=4$salt$hash
	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads, b64Salt, b64Hash)
	return encoded, nil
}

// VerifyPassword verifies a password against an Argon2id hash
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	var m, t, p uint32
	_, err := fmt.Sscanf(parts[3], "v=19$m=%d,t=%d,p=%d", &m, &t, &p)
	
    // Double check format parsing if Sscanf fails or we want consistent behavior
    if len(parts) == 6 && parts[1] == "argon2id" {
        _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p)
    } else {
         return false, fmt.Errorf("invalid hash algorithm or format")
    }
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	keyLength := uint32(len(decodedHash))
	otherHash := argon2.IDKey([]byte(password), salt, t, m, uint8(p), keyLength)

	if len(decodedHash) != len(otherHash) {
		return false, nil
	}
    
    // Constant time comparison
    match := true
    for i := 0; i < len(decodedHash); i++ {
        if decodedHash[i] != otherHash[i] {
            match = false
        }
    }

	return match, nil
}
