package utils

import (
	"crypto"
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cs438/storage"
	"io"
	"regexp"
	"strings"
)

// HashChunk hashes the given chunk and returns the digest along with its hex representation.
func HashChunk(chunk []byte) ([]byte, string) {
	h := crypto.SHA256.New()
	h.Write(chunk)
	hashSlice := h.Sum(nil)
	hashHex := hex.EncodeToString(hashSlice)
	return hashSlice, hashHex
}

// Chunkify converts the given file reader into chunks. Returns a mapping from the chunk hash to the chunk, and the
// metahash.
func Chunkify(chunkSize uint, metafileSep string, reader io.Reader) (map[string][]byte, string, error) {
	buffer := make([]byte, chunkSize)
	chunks := make(map[string][]byte)
	var metafileKeyBytes []byte
	var hashList []string
	for {
		n, err := reader.Read(buffer)
		if err != io.EOF && err != nil {
			return nil, "", err
		}
		// Process the buffer if we were able to read some bytes.
		if n > 0 {
			// Create the hash of the chunk.
			chunkHashBytes, chunkHashHex := HashChunk(buffer[:n])
			// Copy the bytes from the buffer into the chunk map.
			chunks[chunkHashHex] = make([]byte, 0, len(buffer))
			for i := 0; i < n; i++ {
				chunks[chunkHashHex] = append(chunks[chunkHashHex], buffer[i])
			}
			// Add the chunk hash to the ordered list of hashes.
			metafileKeyBytes = append(metafileKeyBytes, chunkHashBytes...)
			hashList = append(hashList, chunkHashHex)
		}
		// Exit if EOF
		if err == io.EOF {
			break
		}
	}
	_, metaFileKey := HashChunk(metafileKeyBytes)
	chunks[metaFileKey] = []byte(strings.Join(hashList, metafileSep))
	return chunks, metaFileKey, nil
}

// GetChunkHashses returns the chunk hashes associated with the metafile of the given metahash.
func GetChunkHashses(blobStore storage.Store, metahash string, metafilesep string) ([]string, error) {
	metafileBytes := blobStore.Get(metahash)
	if metafileBytes == nil {
		return nil, fmt.Errorf("could not extract chunks hashes since the metafile is not in the store")
	}
	metafile := string(metafileBytes)
	return strings.Split(metafile, metafilesep), nil
}

// GetLocalChunks returns the list of local chunks with the given metahash. If a chunk specified in the metafile is
// not in the store, the corresponding chunk in the returned list will be nil.
func GetLocalChunks(blobStore storage.Store, metahash string, metafilesep string) ([][]byte, [][]byte, error) {
	chunkHashes, err := GetChunkHashses(blobStore, metahash, metafilesep)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get local chunks: %w", err)
	}
	var chunks [][]byte
	var chunkHashBytes [][]byte
	for _, chunkHash := range chunkHashes {
		chunk := blobStore.Get(chunkHash)
		chunks = append(chunks, chunk)
		if chunk == nil {
			chunkHashBytes = append(chunkHashBytes, nil)
		} else {
			chunkHashBytes = append(chunkHashBytes, []byte(chunkHash))
		}
	}
	return chunks, chunkHashBytes, nil
}

func IsFullMatch(chunks [][]byte) bool {
	for _, chunk := range chunks {
		if chunk == nil {
			return false
		}
	}
	return true
}

func IsFullMatchLocally(blobStore storage.Store, metahash string, metafilesep string) bool {
	chunks, _, err := GetLocalChunks(blobStore, metahash, metafilesep)
	if err != nil {
		return false
	}
	return IsFullMatch(chunks)
}

func GetMatchedNames(namingStore storage.Store, pattern string) []string {
	var allMatches []string
	namingStore.ForEach(
		func(name string, mh []byte) bool {
			match, _ := regexp.MatchString(pattern, name)
			if match {
				allMatches = append(allMatches, name)
			}
			return true
		})
	return allMatches
}
