package feed

import "fmt"

type Endorsement struct {
	UserID            string
	LastRequestedTime int
	GivenEndorsements int
	EndorsedUsers     map[string]struct{}
}

func NewEndorsementHandler(userID string) Endorsement {
	return Endorsement{
		UserID:            userID,
		LastRequestedTime: -1,
		GivenEndorsements: 0,
		EndorsedUsers:     make(map[string]struct{}, REQUIRED_ENDORSEMENTS),
	}
}

func (e *Endorsement) Copy() Endorsement {
	endorsedUsers := make(map[string]struct{}, len(e.EndorsedUsers))
	for k := range e.EndorsedUsers {
		endorsedUsers[k] = struct{}{}
	}
	return Endorsement{
		UserID:            e.UserID,
		LastRequestedTime: e.LastRequestedTime,
		GivenEndorsements: e.GivenEndorsements,
		EndorsedUsers:     endorsedUsers,
	}
}

func (e *Endorsement) Request(time int) {
	e.LastRequestedTime = time
}

func (e *Endorsement) Reset() {
	e.LastRequestedTime = -1
	e.GivenEndorsements = 0
	e.EndorsedUsers = make(map[string]struct{}, REQUIRED_ENDORSEMENTS)
}

// Update updates the endorsement counter and returns whether enough endorsements were achieved.
func (e *Endorsement) Update(currTime int, endorserID string) bool {
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
	Endorsement
}

type StateProcessor = func(UserState, ContentMetadata) UserState

func NewInitialUserState(userID string) *UserState {
	return &UserState{
		CurrentCredits: INITIAL_CREDITS,
		Username:       DEFAULT_USERNAME,
		Followed:       make(map[string]struct{}),
		Endorsement:    NewEndorsementHandler(userID),
	}
}

func (s *UserState) Copy() UserState {
	followedMap := make(map[string]struct{}, len(s.Followed))
	for k := range s.Followed {
		followedMap[k] = struct{}{}
	}
	return UserState{
		CurrentCredits: s.CurrentCredits,
		Username:       s.Username,
		Followed:       followedMap,
		Endorsement:    s.Endorsement.Copy(),
	}
}

func (s *UserState) Update(metadata ContentMetadata) error {
	// Apply the cost.
	s.CurrentCredits -= metadata.Type.Cost()
	// Update the username.
	if metadata.Type == USERNAME {
		username, err := ExtractUsername(metadata)
		if err != nil {
			return err
		}
		s.Username = username
	}
	// Update the follow list.
	if metadata.Type == FOLLOW {
		targetUser, err := ExtractFollowedUser(metadata)
		if err != nil {
			return err
		}
		s.Followed[targetUser] = struct{}{}
	}
	if metadata.Type == UNFOLLOW {
		targetUser, err := ExtractFollowedUser(metadata)
		if err != nil {
			return err
		}
		delete(s.Followed, targetUser)
	}
	// Handle endorsement request stuff. The given endorsements will be handled outside, since they reside in different
	// blockchains.
	if metadata.Type == ENDORSEMENT_REQUEST {
		s.Endorsement.Request(metadata.Timestamp)
	}
	return nil
}

func (s UserState) IsFollowing(targetUserID string) bool {
	_, ok := s.Followed[targetUserID]
	return ok
}
