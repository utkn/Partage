package consensus

import (
	"fmt"
	"go.dedis.ch/cs438/types"
	"math"
	"sync"
)

type AcceptanceProgress struct {
	Value    types.PaxosValue
	Progress int
}

type TLCProgress struct {
	Block    types.BlockchainBlock
	Progress int
}

type Clock struct {
	Lock          sync.RWMutex
	Step          uint
	MaxID         int
	AcceptedID    uint
	AcceptedValue *types.PaxosValue

	// Maps a paxos value to the number of acceptances received for that value.
	AcceptanceProgressMap map[string]*AcceptanceProgress
	// Maps a step to the number of tlc messages received on that step.
	TLCProgressMap map[int]*TLCProgress
	// A set of step numbers for which a TLC broadcast has occurred.
	TLCBroadcastMap map[int]struct{}
}

func NewClock() *Clock {
	return &Clock{
		Step:                  0,
		MaxID:                 0,
		AcceptanceProgressMap: make(map[string]*AcceptanceProgress),
		TLCProgressMap:        make(map[int]*TLCProgress),
		TLCBroadcastMap:       make(map[int]struct{}),
	}
}

func (c *Clock) ShouldIgnorePropose(receivedStep uint, receivedID int) bool {
	return receivedStep != c.Step || receivedID != c.MaxID
}

func (c *Clock) ShouldIgnorePrepare(receivedStep uint, receivedID int) bool {
	return receivedStep != c.Step || receivedID <= c.MaxID
}

func (c *Clock) InStep(receivedStep uint) bool {
	return receivedStep == c.Step
}

// NotifyAcceptance increments the amount of accept messages received for the given value.
// Returns true iff the progress has reached (or passed) the threshold value.
func (c *Clock) NotifyAcceptance(acceptedValue types.PaxosValue, threshold int) bool {
	prog, ok := c.AcceptanceProgressMap[acceptedValue.UniqID]
	if !ok {
		c.AcceptanceProgressMap[acceptedValue.UniqID] = &AcceptanceProgress{
			Value:    acceptedValue,
			Progress: 0,
		}
		prog = c.AcceptanceProgressMap[acceptedValue.UniqID]
	}
	prog.Progress += 1
	return prog.Progress >= threshold
}

// NotifyTLC increments the amount of TLC messages received in the given step.
func (c *Clock) NotifyTLC(step int, block types.BlockchainBlock) {
	prog, ok := c.TLCProgressMap[step]
	if !ok {
		c.TLCProgressMap[step] = &TLCProgress{
			Block:    block,
			Progress: 0,
		}
		prog = c.TLCProgressMap[step]
	}
	prog.Progress += 1
}

// CatchUp returns an ordered list of blockchain blocks that should be accepted. After invoking this function,
// the associated steps are deleted from the clock.
func (c *Clock) CatchUp(threshold int) []types.BlockchainBlock {
	var newBlocks []types.BlockchainBlock
	stepsToDiscard := map[int]struct{}{}
	// Collect the blocks from completed steps.
	minStep := int(c.Step)
	for i := minStep; i < math.MaxInt; i++ {
		prog, ok := c.TLCProgressMap[i]
		// Once we reach a step where the threshold has not been reached, stop catching up.
		if !ok || prog.Progress < threshold {
			break
		}
		if prog.Progress >= threshold {
			newBlocks = append(newBlocks, prog.Block)
			stepsToDiscard[i] = struct{}{}
		}
		c.AcceptedID = 0
		c.AcceptedValue = nil
		c.MaxID = 0
		c.Step += 1
	}
	// Delete the old steps.
	for step := range stepsToDiscard {
		delete(c.TLCProgressMap, step)
	}
	return newBlocks
}

func (c *Clock) UpdateMaxID(newMaxID int) {
	if newMaxID > c.MaxID {
		c.MaxID = newMaxID
	}
}

func (c *Clock) HasBroadcasted(step int) bool {
	_, ok := c.TLCBroadcastMap[step]
	return ok
}

func (c *Clock) MarkBroadcasted(step int) {
	c.TLCBroadcastMap[step] = struct{}{}
}

func (c *Clock) Accept(acceptedID uint, acceptedValue types.PaxosValue) {
	c.AcceptedID = acceptedID
	c.AcceptedValue = &acceptedValue
}

func (c *Clock) String() string {
	return fmt.Sprintf("[Step=%d, MaxID=%d]", c.Step, c.MaxID)
}
