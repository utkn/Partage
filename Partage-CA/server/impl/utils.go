package impl

import (
	"bytes"
	//"errors"
	"fmt"
	"math/rand"
	"os"
	"partage-ca/server"
	"strings"
)

func LoadUsers() (map[[32]byte]struct{},map[string]struct{}, error) {
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage-CA")]
	fmt.Println("loading taken public keys from", rt+server.UsersPath, "file...")
	windowSize:=32 //bytes
	users := make(map[[32]byte]struct{})
	emails:=make(map[string]struct{})
	data, err := os.ReadFile(rt+server.UsersPath)
	usersCount:=0
	emailsCount:=0
	if err == nil {
		var hash [32]byte
		for i:=0;i<=len(data)-windowSize;i+=windowSize{
			copy(hash[:], data[i:i+windowSize])
			users[hash] = struct{}{}
			usersCount++
		}
	}
	data, err = os.ReadFile(rt+server.EmailsPath)
	if err == nil {
		for _,email := range(bytes.Split(data,[]byte("\n"))){
			if string(email)!=""{
				emails[string(email)] = struct{}{}
				emailsCount++
			}
		}
	}
	fmt.Println("finished loading",usersCount,"user's public key hashes &",emailsCount,"emails !")
	return users, emails,nil
}

func OpenFileToAppend(path string) (*os.File, error) {
	//don't forget to close fp! fp.Close()
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage-CA")]
	file, err := os.OpenFile(rt+path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func AppendToFile(data []byte, fp *os.File) error {
	//if len(data)!=32{
	//	return errors.New("appending invalid public key hash")
	//}
	if _, err := fp.Write(data); err != nil {
		return err
	}
	return nil
}



func GenerateChallenge() int{
	//8 digits integer
	low:=10000000 
	high:=99999999
	return (low+rand.Intn(high-low)) 
}