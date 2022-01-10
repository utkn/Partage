package content

import (
	"encoding/json"
)

type TextPost struct {
	AuthorID  string
	Text      string
	Timestamp int64
}

func NewTextPost(authorID string, text string, timestamp int64) TextPost {
	return TextPost{
		AuthorID:  authorID,
		Text:      text,
		Timestamp: timestamp,
	}
}

func ParseTextPost(postBytes []byte) TextPost {
	var post TextPost
	_ = json.Unmarshal(postBytes, &post)
	return post
}

func UnparseTextPost(value TextPost) []byte {
	b, err := json.Marshal(&value)
	if err != nil {
		return nil
	}
	return b
}

type CommentPost struct {
	AuthorID     string
	Text         string
	Timestamp    int64
	RefContentID string
}

func NewCommentPost(authorID string, text string, timestamp int64, refContentID string) CommentPost {
	return CommentPost{
		AuthorID:     authorID,
		Text:         text,
		Timestamp:    timestamp,
		RefContentID: refContentID,
	}
}

func ParseCommentPost(postBytes []byte) CommentPost {
	var comment CommentPost
	_ = json.Unmarshal(postBytes, &comment)
	return comment
}

func UnparseCommentPost(value CommentPost) []byte {
	b, err := json.Marshal(&value)
	if err != nil {
		return nil
	}
	return b
}
