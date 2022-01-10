package utils

import (
	"crypto"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	"acceptor":      false,
	"proposer":      false,
	"tlc":           false,
	"searchPK":      false,
	"social":        false,
	"statemachine":  false,
	"tls":           false,
}

func PrintDebug(tag string, objs ...interface{}) {
	if DEBUG[tag] {
		fmt.Println("[", strings.ToUpper(tag), "]", objs)
	}
}

func Time() int {
	return 0
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

func HashContentMetadata(index int, uniqID string, contentType string, userID string, contentID string,
	prevHash []byte) []byte {
	h := crypto.SHA256.New()
	h.Write([]byte(strconv.Itoa(index)))
	h.Write([]byte(uniqID))
	h.Write([]byte(contentType))
	h.Write([]byte(userID))
	h.Write([]byte(contentID))
	h.Write(prevHash)
	hashSlice := h.Sum(nil)
	return hashSlice
}
