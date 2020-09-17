package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"os"
)

var (
	keyText = "aabcice12798akljzmknm.ahkjkljl;k"
	commonIV = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
)

func main() {
	var plainText string
	// create encrypt algorithm
	cip, err := aes.NewCipher([]byte(keyText))
	if err != nil {
		panic(err)
	}
	input := bufio.NewScanner(os.Stdin)
	fmt.Printf("Please input your password: ")
	if input.Scan() {
		plainText = input.Text()
	}
	plainTextByte := []byte(plainText)
	// encrypt plaintext
	cfb := cipher.NewCFBEncrypter(cip, commonIV)
	cipherText := make([]byte, len(plainTextByte))
	cfb.XORKeyStream(cipherText, plainTextByte)
	fmt.Printf("Encrypted string is: %s\n", hex.EncodeToString(cipherText))
}
