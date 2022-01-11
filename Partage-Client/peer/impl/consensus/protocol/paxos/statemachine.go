package paxos

import (
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
	"sync"
)

type State interface {
	Next() (State, types.BlockchainBlock)
	Accept(types.Message) bool
	Name() string
}

type StateMachine struct {
	sync.Mutex
	Current State
}

// Run runs the state machine with the given initial state, and returns the output of the last state.
func (m *StateMachine) Run(initialState State) types.BlockchainBlock {
	m.Lock()
	m.Current = initialState
	m.Unlock()
	var finalOutput types.BlockchainBlock
	for {
		nextState, stateOutput := m.Current.Next()
		if nextState == nil {
			utils.PrintDebug("statemachine", "State machine exited.")
			finalOutput = stateOutput
			break
		}
		// Switch the state.
		m.Lock()
		utils.PrintDebug("statemachine", "Switching ", m.Current.Name(), "->", nextState.Name())
		m.Current = nextState
		m.Unlock()
	}
	return finalOutput
}

// Input routes the given message to the current state of the state machine.
func (m *StateMachine) Input(message types.Message) bool {
	m.Lock()
	defer m.Unlock()
	if m.Current == nil {
		utils.PrintDebug("statemachine", "Discarding", message.Name(), "as the state machine is not active.")
		return false
	}
	utils.PrintDebug("statemachine", "Routing", message.Name(), "to", m.Current.Name())
	return m.Current.Accept(message)
}
