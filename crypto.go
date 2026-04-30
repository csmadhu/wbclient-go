package wbclientgo

import (
	"crypto/des"
	"crypto/rand"
	"crypto/sha1"
	"unicode/utf16"

	"golang.org/x/crypto/md4"
)

// MSCHAPv2 cryptographic constants
const (
	// ChallengeLength is the length of authenticator and peer challenges in bytes
	ChallengeLength = 16

	// NTHashLength is the length of NT password hash (MD4) in bytes
	NTHashLength = 16

	// NTResponseLength is the length of NT-Response in bytes
	NTResponseLength = 24

	// AuthResponseLength is the length of authenticator response digest in bytes
	AuthResponseLength = 20

	// ChallengeHashLength is the length of challenge hash in bytes
	ChallengeHashLength = 8
)

// GenerateNTResponseSimple generates a 24-byte NT-Response for MS-CHAPv2 authentication
// using only an 8-byte challenge and password (simplified version for Winbind).
//
// This is a simplified version that doesn't use peer challenge or authenticator challenge,
// just the final 8-byte challenge that Winbind expects.
//
// Parameters:
//   - challenge: 8-byte challenge
//   - password: plaintext password
//
// Returns:
//   - 24-byte NT-Response
func GenerateNTResponseSimple(challenge [ChallengeHashLength]byte, password string) [NTResponseLength]byte {
	// Step 1: Compute NT password hash (MD4 of UTF-16LE encoded password)
	passwordHash := NTHash(password)

	// Step 2: Generate the 24-byte response using DES encryption
	return GenerateChallengeResponse(challenge, passwordHash)
}

// NTHash computes the NT password hash (MD4 of UTF-16LE encoded password)
func NTHash(password string) [16]byte {
	// Convert password to UTF-16LE encoding
	utf16Password := EncodeUTF16LE(password)

	// Compute MD4 hash
	mdfour := md4.New()
	mdfour.Write(utf16Password)

	var hash [16]byte
	copy(hash[:], mdfour.Sum(nil))
	return hash
}

// EncodeUTF16LE encodes a string to UTF-16LE byte array.
// This is used for MSCHAPv2 password encoding as specified in RFC 2759.
func EncodeUTF16LE(s string) []byte {
	// Convert string to UTF-16 code points
	utf16Codes := utf16.Encode([]rune(s))

	// Convert to little-endian byte array
	result := make([]byte, len(utf16Codes)*2)
	for i, code := range utf16Codes {
		result[i*2] = byte(code)        // Low byte
		result[i*2+1] = byte(code >> 8) // High byte
	}

	return result
}

// GenerateChallengeResponse generates the 24-byte NT-Response using challenge and password hash
func GenerateChallengeResponse(challenge [ChallengeHashLength]byte, passwordHash [NTHashLength]byte) [NTResponseLength]byte {
	// Zero-pad PasswordHash to 21 octets
	var zPasswordHash [21]byte
	copy(zPasswordHash[:NTHashLength], passwordHash[:])
	// Remaining 5 bytes are already zero from array initialization

	// Split into three 7-byte keys
	var key1, key2, key3 [7]byte
	copy(key1[:], zPasswordHash[0:7])
	copy(key2[:], zPasswordHash[7:14])
	copy(key3[:], zPasswordHash[14:21])

	// Encrypt challenge with each key using DES
	response1 := DesEncrypt(challenge, key1)
	response2 := DesEncrypt(challenge, key2)
	response3 := DesEncrypt(challenge, key3)

	// Concatenate the three 8-byte results to form NTResponseLength-byte response
	// Use append to combine the three arrays into a single slice
	responseSlice := append(response1[:], response2[:]...)
	responseSlice = append(responseSlice, response3[:]...)

	// Convert the slice to a fixed-size array
	var response [NTResponseLength]byte
	copy(response[:], responseSlice)

	return response
}

// DesEncrypt encrypts clear text using DES with the given 7-byte key
func DesEncrypt(clear [8]byte, key [7]byte) [8]byte {
	// Expand the 7-byte key to 8 bytes by inserting parity bits
	desKey := expandKey(key)

	// Create DES cipher
	block, err := des.NewCipher(desKey[:])
	if err != nil {
		// This should never happen with a valid 8-byte key
		panic(err)
	}

	// Encrypt the clear text (DES ECB mode)
	var cypher [8]byte
	block.Encrypt(cypher[:], clear[:])

	return cypher
}

// expandKey expands a 7-byte key to 8 bytes (DES key expansion)
func expandKey(key [7]byte) [8]byte {
	var expanded [8]byte

	// This matches FreeRADIUS smbdes.c str_to_key() exactly
	// Extract 56 bits from 7 bytes into 8 bytes (7 bits per byte)
	expanded[0] = key[0] >> 1
	expanded[1] = ((key[0] & 0x01) << 6) | (key[1] >> 2)
	expanded[2] = ((key[1] & 0x03) << 5) | (key[2] >> 3)
	expanded[3] = ((key[2] & 0x07) << 4) | (key[3] >> 4)
	expanded[4] = ((key[3] & 0x0F) << 3) | (key[4] >> 5)
	expanded[5] = ((key[4] & 0x1F) << 2) | (key[5] >> 6)
	expanded[6] = ((key[5] & 0x3F) << 1) | (key[6] >> 7)
	expanded[7] = key[6] & 0x7F

	// Shift left by 1 (positions parity bit at bit 0)
	for i := 0; i < 8; i++ {
		expanded[i] = expanded[i] << 1
	}

	return expanded
}


// GenerateChallengeHash generates the 8-byte challenge hash for MS-CHAPv2
// Per RFC 2759 Section 8.2: ChallengeHash = first 8 bytes of SHA1(PeerChallenge || AuthenticatorChallenge || UserName)
func GenerateChallengeHash(peerChallenge, authenticatorChallenge []byte, userName string) [8]byte {
	hasher := sha1.New()
	hasher.Write(peerChallenge)
	hasher.Write(authenticatorChallenge)
	hasher.Write([]byte(userName))
	hash := hasher.Sum(nil)

	var challengeHash [8]byte
	copy(challengeHash[:], hash[:8])
	return challengeHash
}

// GenerateRandomChallenge generates a random 8-byte challenge
func GenerateRandomChallenge() ([8]byte, error) {
	var challenge [8]byte
	_, err := rand.Read(challenge[:])
	return challenge, err
}
