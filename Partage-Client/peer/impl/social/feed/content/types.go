package content

import (
	"encoding/hex"
	"fmt"
	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer/impl/utils"
)

type ContentType int

const (
	UNKNOWN ContentType = iota
	TEXT
	COMMENT
	REACTION
	FOLLOW
	UNFOLLOW
	USERNAME
	ENDORSEMENT
	ENDORSEMENT_REQUEST
)

func (c ContentType) String() string {
	switch c {
	case UNKNOWN:
		return "unknown"
	case TEXT:
		return "text"
	case COMMENT:
		return "comment"
	case REACTION:
		return "reaction"
	case FOLLOW:
		return "follow"
	case UNFOLLOW:
		return "unfollow"
	case USERNAME:
		return "username"
	case ENDORSEMENT:
		return "endorsement"
	case ENDORSEMENT_REQUEST:
		return "endorsement_request"
	}
	return "undefined"
}

func (c ContentType) Cost() int {
	switch c {
	case TEXT:
		return 5
	case COMMENT:
		return 1
	case REACTION:
		return 1
	}
	return 0
}

func CreateChangeUsernameMetadata(userID string, newUsername string) Metadata {
	return Metadata{
		FeedUserID: userID,
		Type:       USERNAME,
		ContentID:  "",
		Timestamp:  utils.Time(),
		Data:       []byte(newUsername),
		Signature:  nil,
	}
}

func CreateFollowUserMetadata(userID string, targetUserID string, unfollow bool) Metadata {
	t := FOLLOW
	if unfollow {
		t = UNFOLLOW
	}
	data, _ := hex.DecodeString(targetUserID)
	return Metadata{
		FeedUserID: userID,
		Type:       t,
		ContentID:  "",
		Timestamp:  utils.Time(),
		Data:       data,
		Signature:  nil,
	}
}

func CreateEndorseUserMetadata(userID string, targetUserID string) Metadata {
	data, _ := hex.DecodeString(targetUserID)
	return Metadata{
		FeedUserID: userID,
		Type:       ENDORSEMENT,
		ContentID:  "",
		Timestamp:  utils.Time(),
		Data:       data,
		Signature:  nil,
	}
}

func CreateEndorsementRequestMetadata(userID string) Metadata {
	return Metadata{
		FeedUserID: userID,
		Type:       ENDORSEMENT_REQUEST,
		ContentID:  "",
		Timestamp:  utils.Time(),
		Data:       nil,
		Signature:  nil,
	}
}

func CreateTextMetadata(userID string, metahash string) Metadata {
	// Create a random content id.
	contentID := xid.New().String()
	return Metadata{
		FeedUserID: userID,
		Type:       TEXT,
		ContentID:  contentID,
		Timestamp:  utils.Time(),
		Data:       []byte(metahash),
		Signature:  nil,
	}
}

// ParseUsername extracts the username string from a USERNAME metadata object.
func ParseUsername(metadata Metadata) (string, error) {
	if metadata.Type != USERNAME {
		return "", fmt.Errorf("cannot extract the username from a non-username metadata")
	}
	return string(metadata.Data), nil
}

// ParseFollowedUser extracts the target followed user id string from a FOLLOW or UNFOLLOW metadata object.
func ParseFollowedUser(metadata Metadata) (string, error) {
	if metadata.Type != FOLLOW && metadata.Type != UNFOLLOW {
		return "", fmt.Errorf("cannot extract the followed user id from a non-follow metadata")
	}
	return hex.EncodeToString(metadata.Data), nil
}

// ParseEndorsedUserID extracts the target endorsed user id string from a ENDORSEMENT metadata object.
func ParseEndorsedUserID(metadata Metadata) (string, error) {
	if metadata.Type != ENDORSEMENT {
		return "", fmt.Errorf("cannot extract the endorsed user id from non-endorsement metadata")
	}
	return hex.EncodeToString(metadata.Data), nil
}

// ParseTextPostMetadata extracts the metahash for the text object from a TEXT metadata object.
func ParseTextPostMetadata(metadata Metadata) (string, error) {
	if metadata.Type != TEXT {
		return "", fmt.Errorf("cannot extract the metahash from non-text metadata")
	}
	return string(metadata.Data), nil
}
