package impl

import (
	"go.dedis.ch/cs438/peer/impl/social/feed"
	"go.dedis.ch/cs438/peer/impl/utils"
)

type UserData struct {
	Username             string
	UserID               string
	Followers            []string
	Followees            []string
	CanBeEndorsed        bool
	ReceivedEndorsements int
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
		Username:             userState.Username,
		UserID:               userState.UserID,
		Followers:            followers,
		Followees:            followees,
		CanBeEndorsed:        userState.CanEndorse(utils.Time(), selfUserID),
		ReceivedEndorsements: userState.ReceivedEndorsements,
	}
}

type Reaction struct {
	AuthorID     string
	RefContentID string
	ReactionText string
	BlockHash    string
	Timestamp    int64
}

func NewReaction(info feed.ReactionInfo) Reaction {
	return Reaction{
		AuthorID:     info.FeedUserID,
		RefContentID: info.RefContentID,
		ReactionText: info.Reaction.String(),
		BlockHash:    info.BlockHash,
		Timestamp:    info.Timestamp,
	}
}

type Post struct {
	AuthorID  string
	ContentID string
	Text      string
	BlockHash string
	Timestamp int64
	Reactions []Reaction
	Comments  []Comment
}

func NewPost(text string, c feed.Content, reactions []Reaction, comments []Comment) Post {
	return Post{
		AuthorID:  c.FeedUserID,
		ContentID: c.ContentID,
		Text:      text,
		BlockHash: c.BlockHash,
		Timestamp: c.Timestamp,
		Reactions: reactions,
		Comments:  comments,
	}
}

type Comment struct {
	AuthorID     string
	ContentID    string
	Text         string
	RefContentID string
	BlockHash    string
	Timestamp    int64
	Reactions    []Reaction
}

func NewComment(text string, c feed.Content, reactions []Reaction) Comment {
	return Comment{
		AuthorID:     c.FeedUserID,
		ContentID:    c.ContentID,
		Text:         text,
		RefContentID: c.RefContentID,
		BlockHash:    c.BlockHash,
		Timestamp:    c.Timestamp,
		Reactions:    reactions,
	}
}
