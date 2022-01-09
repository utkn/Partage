package paxos

import (
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/types"
	"sync"
)

type State interface {
	Next() State
	Accept(types.Message) bool
	Name() string
}

type StateMachine struct {
	sync.Mutex
	Current State
}

// Run runs the state machine with the given initial state, and returns the last state before termination.
func (m *StateMachine) Run(initialState State) State {
	m.Lock()
	m.Current = initialState
	m.Unlock()
	for {
		nextState := m.Current.Next()
		if nextState == nil {
			utils.PrintDebug("statemachine", "State machine exited.")
			break
		}
		// Switch the state.
		m.Lock()
		utils.PrintDebug("statemachine", "Switching ", m.Current.Name(), "->", nextState.Name())
		m.Current = nextState
		m.Unlock()
	}
	return m.Current
}

// Input routes the given message to the current state of the state machine.
func (m *StateMachine) Input(message types.Message) bool {
	m.Lock()
	defer m.Unlock()
	utils.PrintDebug("statemachine", "Discarding", message.Name(), "as the state machine is not active.")
	if m.Current == nil {
		return false
	}
	utils.PrintDebug("statemachine", "Routing", message.Name(), "to", m.Current.Name())
	return m.Current.Accept(message)
}
