package hap

import (
	"code.google.com/p/go.crypto/sha3"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
)

const ResponseLength = 128

func hash(in []byte) []byte {
	h := sha3.NewKeccak512()
	h.Write(in)
	return h.Sum(nil)
}

func generateChallenge() string {
	max := big.NewInt(math.MaxInt64)
	max.Mul(max, big.NewInt(2))
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		fmt.Println("[!] failed to generate random number:",
			err.Error())
		os.Exit(1)
	}
	res := uint64(n.Int64())
	return fmt.Sprintf("%d", res)
}

func Response(password, challenge string) []byte {
	response := hash([]byte(password + challenge))
	return response
}

func matchHash(hash1, hash2 []byte) bool {
	var size = len(hash1)
	if size > len(hash2) {
		size = len(hash2)
	}

	var matched = 0
	for i := 0; i < size; i++ {
		matched += subtle.ConstantTimeByteEq(hash1[i], hash2[i])
	}

	match := (matched == size)
	sameSize := len(hash1) == len(hash2)
	return match && sameSize
}

func Validate(password, challenge string, response []byte) bool {
	valid := Response(password, challenge)
	return matchHash(valid, response)
}

func Challenge(conn net.Conn, password string) (err error) {
	challenge := generateChallenge()
	_, err = conn.Write([]byte(challenge))
	if err != nil {
		return
	}

	response := make([]byte, ResponseLength)
	n, err := conn.Read(response)
	if err != nil {
		return
	} else {
		response = response[:n]
	}

	if !Validate(password, challenge, response) {
		return
	}

	conn.Write([]byte("ok"))
	return
}

func Authenticate(conn net.Conn, password string) (err error) {
	challenge := make([]byte, ResponseLength)
	n, err := conn.Read(challenge)
	if err != nil {
		return
	} else {
		challenge = challenge[:n]
	}

	response := Response(password, string(challenge))
	_, err = conn.Write([]byte(response))
	if err != nil {
		return
	}

	ok := make([]byte, 2)
	_, err = conn.Read(ok)
	if err != nil {
		return
	}
	return
}
