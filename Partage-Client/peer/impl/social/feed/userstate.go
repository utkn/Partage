package feed

import (
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
)

type EndorsementHandler struct {
	UserID               string
	LastRequestedTime    int64
	ReceivedEndorsements int
	EndorsedUsers        map[string]struct{}
}

func NewEndorsementHandler(userID string) EndorsementHandler {
	return EndorsementHandler{
		UserID:               userID,
		LastRequestedTime:    -1,
		ReceivedEndorsements: 0,
		EndorsedUsers:        make(map[string]struct{}, REQUIRED_ENDORSEMENTS),
	}
}

func (e *EndorsementHandler) Copy() EndorsementHandler {
	endorsedUsers := make(map[string]struct{}, len(e.EndorsedUsers))
	for k := range e.EndorsedUsers {
		endorsedUsers[k] = struct{}{}
	}
	return EndorsementHandler{
		UserID:               e.UserID,
		LastRequestedTime:    e.LastRequestedTime,
		ReceivedEndorsements: e.ReceivedEndorsements,
		EndorsedUsers:        endorsedUsers,
	}
}

func (e *EndorsementHandler) Request(time int64) {
	if !e.CanRequest() {
		return
	}
	e.LastRequestedTime = time
}

func (e *EndorsementHandler) Reset() {
	e.LastRequestedTime = -1
	e.ReceivedEndorsements = 0
	e.EndorsedUsers = make(map[string]struct{}, REQUIRED_ENDORSEMENTS)
}

// CanRequest returns true if the user can request an endorsement.
// For now, a user can always initiate a new endorsement request as long as their credits are within range,
// overriding the last request.
func (e EndorsementHandler) CanRequest() bool {
	return true
}

func (e EndorsementHandler) CanEndorse(currTime int64, endorserID string) bool {
	// If there is no current endorsement request going on, no endorsement possible.
	if e.LastRequestedTime < 0 {
		return false
	}
	// If the endorser is the user itself, no endorsement possible.
	if endorserID == e.UserID {
		utils.PrintDebug("social", "self-endorsement not allowed")
		return false
	}
	// If the user already endorsed, no endorsement possible.
	_, alreadyEndorsed := e.EndorsedUsers[endorserID]
	if alreadyEndorsed {
		utils.PrintDebug("social", "multi-endorsement not allowed")
		return false
	}
	// Make sure that the endorsement time is valid.
	if currTime-e.LastRequestedTime >= ENDORSEMENT_INTERVAL {
		return false
	}
	return true
}

// ReceiveEndorsement tries to update the endorsement counter and returns whether enough endorsements were achieved.
func (e *EndorsementHandler) ReceiveEndorsement(currTime int64, endorserID string) bool {
	// If the endorsement is invalid, do not update.
	if !e.CanEndorse(currTime, endorserID) {
		return false
	}
	e.EndorsedUsers[endorserID] = struct{}{}
	e.ReceivedEndorsements += 1
	if e.ReceivedEndorsements >= REQUIRED_ENDORSEMENTS {
		e.Reset()
		return true
	}
	return false
}

type UserState struct {
	CurrentCredits int
	Username       string
	// Set of users followed by this user.
	Followees map[string]struct{}
	// Set of users that follow this user.
	Followers map[string]struct{}
	EndorsementHandler
}

func NewInitialUserState(userID string) *UserState {
	return &UserState{
		CurrentCredits:     INITIAL_CREDITS,
		Username:           DEFAULT_USERNAME,
		Followees:          make(map[string]struct{}),
		Followers:          make(map[string]struct{}),
		EndorsementHandler: NewEndorsementHandler(userID),
	}
}

func (s *UserState) Copy() UserState {
	followees := make(map[string]struct{}, len(s.Followees))
	for k := range s.Followees {
		followees[k] = struct{}{}
	}
	followers := make(map[string]struct{}, len(s.Followers))
	for k := range s.Followers {
		followers[k] = struct{}{}
	}
	return UserState{
		CurrentCredits:     s.CurrentCredits,
		Username:           s.Username,
		Followees:          followees,
		Followers:          followers,
		EndorsementHandler: s.EndorsementHandler.Copy(),
	}
}

// Undo removes the effects that was caused by the given metadata when possible. However, credits are not refunded.
func (s *UserState) Undo(metadata content.Metadata) error {
	// Undo the followee list.
	if metadata.Type == content.FOLLOW {
		targetUser, err := content.ParseFollowedUser(metadata)
		if err != nil {
			return err
		}
		delete(s.Followees, targetUser)
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
		s.Followees[targetUser] = struct{}{}
	}
	// Handle endorsement request stuff. The given endorsements will be handled outside, since they reside in different
	// blockchains.
	if metadata.Type == content.ENDORSEMENT_REQUEST {
		s.EndorsementHandler.Request(metadata.Timestamp)
	}
	return nil
}

func (s *UserState) AddFollower(userID string) {
	s.Followers[userID] = struct{}{}
}

func (s *UserState) RemoveFollower(userID string) {
	delete(s.Followers, userID)
}

func (s UserState) IsFollowing(targetUserID string) bool {
	_, ok := s.Followees[targetUserID]
	return ok
}

func (s UserState) IsFollowedBy(targetUserID string) bool {
	_, ok := s.Followers[targetUserID]
	return ok
}
