package internal

import (
	"crypto/des"
	"crypto/rand"
	"crypto/sha1"
	"unicode/utf16"

	"golang.org/x/crypto/md4"
)

const (
	ChallengeLength     = 16
	NTHashLength        = 16
	NTResponseLength    = 24
	AuthResponseLength  = 20
	ChallengeHashLength = 8
)

func GenerateNTResponseSimple(challenge [ChallengeHashLength]byte, password string) [NTResponseLength]byte {
	passwordHash := NTHash(password)
	return GenerateChallengeResponse(challenge, passwordHash)
}

func NTHash(password string) [16]byte {
	utf16Password := EncodeUTF16LE(password)
	mdfour := md4.New()
	mdfour.Write(utf16Password)
	var hash [16]byte
	copy(hash[:], mdfour.Sum(nil))
	return hash
}

func EncodeUTF16LE(s string) []byte {
	utf16Codes := utf16.Encode([]rune(s))
	result := make([]byte, len(utf16Codes)*2)
	for i, code := range utf16Codes {
		result[i*2] = byte(code)
		result[i*2+1] = byte(code >> 8)
	}
	return result
}

func GenerateChallengeResponse(challenge [ChallengeHashLength]byte, passwordHash [NTHashLength]byte) [NTResponseLength]byte {
	var zPasswordHash [21]byte
	copy(zPasswordHash[:NTHashLength], passwordHash[:])

	var key1, key2, key3 [7]byte
	copy(key1[:], zPasswordHash[0:7])
	copy(key2[:], zPasswordHash[7:14])
	copy(key3[:], zPasswordHash[14:21])

	response1 := DesEncrypt(challenge, key1)
	response2 := DesEncrypt(challenge, key2)
	response3 := DesEncrypt(challenge, key3)

	responseSlice := append(response1[:], response2[:]...)
	responseSlice = append(responseSlice, response3[:]...)

	var response [NTResponseLength]byte
	copy(response[:], responseSlice)
	return response
}

func DesEncrypt(clear [8]byte, key [7]byte) [8]byte {
	desKey := expandKey(key)
	block, err := des.NewCipher(desKey[:])
	if err != nil {
		panic(err)
	}
	var cypher [8]byte
	block.Encrypt(cypher[:], clear[:])
	return cypher
}

func expandKey(key [7]byte) [8]byte {
	var expanded [8]byte
	expanded[0] = key[0] >> 1
	expanded[1] = ((key[0] & 0x01) << 6) | (key[1] >> 2)
	expanded[2] = ((key[1] & 0x03) << 5) | (key[2] >> 3)
	expanded[3] = ((key[2] & 0x07) << 4) | (key[3] >> 4)
	expanded[4] = ((key[3] & 0x0F) << 3) | (key[4] >> 5)
	expanded[5] = ((key[4] & 0x1F) << 2) | (key[5] >> 6)
	expanded[6] = ((key[5] & 0x3F) << 1) | (key[6] >> 7)
	expanded[7] = key[6] & 0x7F
	for i := 0; i < 8; i++ {
		expanded[i] = expanded[i] << 1
	}
	return expanded
}

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

func GenerateRandomChallenge() ([8]byte, error) {
	var challenge [8]byte
	_, err := rand.Read(challenge[:])
	return challenge, err
}
