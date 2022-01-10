package content

import (
	"bytes"
	"encoding/json"
	"io"
)

type TextPost struct {
	Text string
}

func NewTextPost(text string) io.Reader {
	value := TextPost{Text: text}
	b, err := json.Marshal(&value)
	if err != nil {
		return nil
	}
	return bytes.NewReader(b)
}

func ParseTextPost(postBytes []byte) TextPost {
	var post TextPost
	_ = json.Unmarshal(postBytes, &post)
	return post
}

type CommentPost struct {
	Text string
}

func NewCommentPost(text string) io.Reader {
	value := CommentPost{Text: text}
	b, err := json.Marshal(&value)
	if err != nil {
		return nil
	}
	return bytes.NewReader(b)
}

func ParseCommentPost(postBytes []byte) CommentPost {
	var comment CommentPost
	_ = json.Unmarshal(postBytes, &comment)
	return comment
}
