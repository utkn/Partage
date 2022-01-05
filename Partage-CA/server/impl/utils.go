package impl

import (
	"errors"
	"fmt"
	"os"
	"partage-ca/server"
	"strings"
)

func LoadUsers() (map[[32]byte]struct{}, error) {
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage-CA")]
	fmt.Println("loading taken public keys from", rt+server.UsersPath, "file...")
	windowSize:=32 //bytes
	users := make(map[[32]byte]struct{})
	data, err := os.ReadFile(rt+server.UsersPath)
	if err != nil {
		return users,nil
	}
	usersCount:=0
	var hash [32]byte
	for i:=0;i<=len(data)-windowSize;i+=windowSize{
		copy(hash[:], data[i:i+windowSize])
		users[hash] = struct{}{}
		usersCount++
	}
	fmt.Println("finished loading",usersCount,"user's public key hashes!")
	return users, nil
}

func OpenFileToAppend() (*os.File, error) {
	//don't forget to close fp! fp.Close()
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage-CA")]
	file, err := os.OpenFile(rt+server.UsersPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func AppendToFile(data []byte, fp *os.File) error {
	if len(data)!=32{
		return errors.New("appending invalid public key hash")
	}
	if _, err := fp.Write(data); err != nil {
		return err
	}
	return nil
}