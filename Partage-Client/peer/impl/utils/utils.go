package utils

import (
	"crypto"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

var DEBUG = map[string]bool{
	"network":       false,
	"communication": false,
	"handler":       false,
	"antientropy":   false,
	"heartbeat":     false,
	"messaging":     false,
	"gossip":        false,
	"data":          false,
	"acceptor":      true,
	"proposer":      false,
	"tlc":           false,
	"searchPK":      false,
	"social":        false,
	"statemachine":  false,
	"tls":           true,
}

func PrintDebug(tag string, objs ...interface{}) {
	if DEBUG[tag] {
		fmt.Println("[", strings.ToUpper(tag), "]", objs)
	}
}

func Time() int64 {
	return time.Now().UTC().Unix()
}

func ChooseRandom(options map[string]struct{}, exclusion map[string]struct{}) (string, error) {
	for opt := range options {
		if exclusion == nil {
			return opt, nil
		}
		_, isExcluded := exclusion[opt]
		if !isExcluded {
			return opt, nil
		}
	}
	return "", errors.New("no possible choice")
}

// DistributeBudget takes a total budget and a list of neighbors, and distributes the budget as evenly as possible.
// Returns a mapping from a neighbor to its non-zero budget. Neighbors with a zero budget are omitted from the map.
func DistributeBudget(budget uint, neighbors map[string]struct{}) map[string]uint {
	if len(neighbors) == 0 {
		return nil
	}
	qtn := int(budget) / len(neighbors)
	rem := int(budget) % len(neighbors)
	budgetMap := make(map[string]uint, len(neighbors))
	i := 0
	for neighbor := range neighbors {
		neighborBudget := qtn
		if i < rem {
			neighborBudget += 1
		}
		budgetMap[neighbor] = uint(neighborBudget)
		i += 1
	}
	// Only keep the neighbors with a non-zero budget.
	budgetMapNonZero := make(map[string]uint, len(budgetMap))
	for neighbor, budget := range budgetMap {
		if budget > 0 {
			budgetMapNonZero[neighbor] = budget
		}
	}
	return budgetMapNonZero
}

func HashNameBlock(index int, uniqID string, fileName string, metahash string, prevHash []byte) []byte {
	h := crypto.SHA256.New()
	h.Write([]byte(strconv.Itoa(index)))
	h.Write([]byte(uniqID))
	h.Write([]byte(fileName))
	h.Write([]byte(metahash))
	h.Write(prevHash)
	hashSlice := h.Sum(nil)
	return hashSlice
}

func OpenFileToAppend(path string) (*os.File, error) {
	//don't forget to close fp! fp.Close()
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage")]
	file, err := os.OpenFile(rt+path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func OpenFileToWrite(path string) (*os.File, error) {
	//don't forget to close fp! fp.Close()
	wd, _ := os.Getwd()
	rt := wd[:strings.Index(wd, "Partage")]
	file, err := os.OpenFile(rt+path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func AppendToFile(data []byte, fp *os.File) error {
	if _, err := fp.Write(data); err != nil {
		return err
	}
	return nil
}
