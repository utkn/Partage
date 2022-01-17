package impl

import (
	"time"

	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/utils"
)

type UserData struct {
	Username               string
	UserID                 string
	Followers              []string
	Followees              []string
	CanBeEndorsed          bool
	CanRequestEndorsements bool
	ReceivedEndorsements   int
	Credits                int
}

type Reaction struct {
	Author          UserData
	RefContentID    string
	ReactionText    string
	BlockHash       string
	Timestamp       int64
	TimestampToDate func(int64) string
}

type Comment struct {
	Author          UserData
	ContentID       string
	Text            string
	RefContentID    string
	BlockHash       string
	Timestamp       int64
	Reactions       []Reaction
	AlreadyReacted  string
	TimestampToDate func(int64) string
}

type Text struct {
	Author          UserData
	ContentID       string
	Text            string
	BlockHash       string
	Timestamp       int64
	Reactions       []Reaction
	Comments        []Comment
	Recipients      []string
	AlreadyReacted  string
	TimestampToDate func(int64) string
}

func NewUserData(selfUserID string, userState feed.UserState) UserData {
	var followers []string
	for f := range userState.Followers {
		followers = append(followers, f)
	}
	var followees []string
	for f := range userState.Followees {
		followees = append(followees, f)
	}
	return UserData{
		Username:               userState.Username,
		UserID:                 userState.UserID,
		Credits:                userState.CurrentCredits,
		Followers:              followers,
		Followees:              followees,
		CanRequestEndorsements: userState.CanRequest(utils.Time()),
		CanBeEndorsed:          userState.CanEndorse(utils.Time(), selfUserID),
		ReceivedEndorsements:   userState.ReceivedEndorsements,
	}
}

func NewReaction(info feed.ReactionInfo, author UserData) Reaction {
	return Reaction{
		Author:          author,
		RefContentID:    info.RefContentID,
		ReactionText:    info.Reaction.String(),
		BlockHash:       info.BlockHash,
		Timestamp:       info.Timestamp,
		TimestampToDate: timestampToDate,
	}
}

func NewComment(text string, c feed.Content, author UserData, reactions []Reaction) Comment {
	return Comment{
		Author:          author,
		ContentID:       c.ContentID,
		Text:            text,
		RefContentID:    c.RefContentID,
		BlockHash:       c.BlockHash,
		Timestamp:       c.Timestamp,
		Reactions:       reactions,
		TimestampToDate: timestampToDate,
	}
}

func NewText(text string, c feed.Content, author UserData, reactions []Reaction, comments []Comment) Text {
	return Text{
		Author:          author,
		ContentID:       c.ContentID,
		Text:            text,
		BlockHash:       c.BlockHash,
		Timestamp:       c.Timestamp,
		Reactions:       reactions,
		Comments:        comments,
		TimestampToDate: timestampToDate,
	}
}

func timestampToDate(d int64) string {
	return time.Unix(d, 0).Format("15:04:05 2006-01-02 ")
}
