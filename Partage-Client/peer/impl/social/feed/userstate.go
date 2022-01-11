package feed

import (
	"fmt"
	"go.dedis.ch/cs438/peer/impl/content"
)

type EndorsementHandler struct {
	UserID            string
	LastRequestedTime int64
	GivenEndorsements int
	EndorsedUsers     map[string]struct{}
}

func NewEndorsementHandler(userID string) EndorsementHandler {
	return EndorsementHandler{
		UserID:            userID,
		LastRequestedTime: -1,
		GivenEndorsements: 0,
		EndorsedUsers:     make(map[string]struct{}, REQUIRED_ENDORSEMENTS),
	}
}

func (e *EndorsementHandler) Copy() EndorsementHandler {
	endorsedUsers := make(map[string]struct{}, len(e.EndorsedUsers))
	for k := range e.EndorsedUsers {
		endorsedUsers[k] = struct{}{}
	}
	return EndorsementHandler{
		UserID:            e.UserID,
		LastRequestedTime: e.LastRequestedTime,
		GivenEndorsements: e.GivenEndorsements,
		EndorsedUsers:     endorsedUsers,
	}
}

func (e *EndorsementHandler) Request(time int64) {
	e.LastRequestedTime = time
}

func (e *EndorsementHandler) Reset() {
	e.LastRequestedTime = -1
	e.GivenEndorsements = 0
	e.EndorsedUsers = make(map[string]struct{}, REQUIRED_ENDORSEMENTS)
}

// Update tries to update the endorsement counter and returns whether enough endorsements were achieved.
func (e *EndorsementHandler) Update(currTime int64, endorserID string) bool {
	// If there is no current endorsement request going on, do nothing.
	if e.LastRequestedTime < 0 {
		return false
	}
	// If the endorser is the user itself, do nothing.
	if endorserID == e.UserID {
		fmt.Println("self-endorsement not allowed")
		return false
	}
	// If the user already endorsed, do nothing.
	_, alreadyEndorsed := e.EndorsedUsers[endorserID]
	if alreadyEndorsed {
		fmt.Println("multi-endorsement not allowed")
		return false
	}
	// Make sure that the endorsement is valid. If not, invalidate it.
	if currTime-e.LastRequestedTime >= ENDORSEMENT_INTERVAL {
		e.Reset()
		return false
	}
	e.EndorsedUsers[endorserID] = struct{}{}
	e.GivenEndorsements += 1
	if e.GivenEndorsements >= REQUIRED_ENDORSEMENTS {
		e.Reset()
		return true
	}
	return false
}

type UserState struct {
	CurrentCredits int
	Username       string
	Followed       map[string]struct{}
	EndorsementHandler
}

type StateProcessor = func(UserState, content.Metadata) UserState

func NewInitialUserState(userID string) *UserState {
	return &UserState{
		CurrentCredits:     INITIAL_CREDITS,
		Username:           DEFAULT_USERNAME,
		Followed:           make(map[string]struct{}),
		EndorsementHandler: NewEndorsementHandler(userID),
	}
}

func (s *UserState) Copy() UserState {
	followedMap := make(map[string]struct{}, len(s.Followed))
	for k := range s.Followed {
		followedMap[k] = struct{}{}
	}
	return UserState{
		CurrentCredits:     s.CurrentCredits,
		Username:           s.Username,
		Followed:           followedMap,
		EndorsementHandler: s.EndorsementHandler.Copy(),
	}
}

// Undo removes the effects that was caused by the given metadata when possible. However, credits are not refunded.
func (s *UserState) Undo(metadata content.Metadata) error {
	// Undo the follow list.
	if metadata.Type == content.FOLLOW {
		targetUser, err := content.ParseFollowedUser(metadata)
		if err != nil {
			return err
		}
		delete(s.Followed, targetUser)
	}
	return nil
}

// Update updates the user state with the given metadata.
func (s *UserState) Update(metadata content.Metadata) error {
	// Apply the cost.
	s.CurrentCredits -= metadata.Type.Cost()
	// Update the username.
	if metadata.Type == content.USERNAME {
		username, err := content.ParseUsername(metadata)
		if err != nil {
			return err
		}
		s.Username = username
	}
	// Update the follow list.
	if metadata.Type == content.FOLLOW {
		targetUser, err := content.ParseFollowedUser(metadata)
		if err != nil {
			return err
		}
		s.Followed[targetUser] = struct{}{}
	}
	// Handle endorsement request stuff. The given endorsements will be handled outside, since they reside in different
	// blockchains.
	// Only process an endorsement request if the user's credit is lower than the set amount.
	if metadata.Type == content.ENDORSEMENT_REQUEST && s.CurrentCredits <= ENDORSEMENT_REQUEST_CREDIT_LIMIT {
		s.EndorsementHandler.Request(metadata.Timestamp)
	}
	return nil
}

func (s UserState) IsFollowing(targetUserID string) bool {
	_, ok := s.Followed[targetUserID]
	return ok
}
