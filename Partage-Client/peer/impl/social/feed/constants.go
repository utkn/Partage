package feed

import (
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cs438/peer/impl/utils"
)

var INITIAL_CREDITS = 10
var DEFAULT_USERNAME = "Rambo"
var REQUIRED_ENDORSEMENTS = 5
var ENDORSEMENT_INTERVAL = 60 * 60
var ENDORSEMENT_REWARD = 100
var ENDORSEMENT_REQUEST_CREDIT_LIMIT = 20

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

// ExtractUsername extracts the username string from a USERNAME metadata object.
func ExtractUsername(metadata ContentMetadata) (string, error) {
	if metadata.Type != USERNAME {
		return "", fmt.Errorf("cannot extract the username from a non-username metadata")
	}
	return string(metadata.Data), nil
}

func CreateChangeUsernameMetadata(userID string, newUsername string) ContentMetadata {
	return ContentMetadata{
		FeedUserID: userID,
		Type:       USERNAME,
		ContentID:  "",
		Timestamp:  utils.Time(),
		Data:       []byte(newUsername),
		Signature:  nil,
	}
}

// ExtractFollowedUser extracts the target followed user id string from a FOLLOW or UNFOLLOW metadata object.
func ExtractFollowedUser(metadata ContentMetadata) (string, error) {
	if metadata.Type != FOLLOW && metadata.Type != UNFOLLOW {
		return "", fmt.Errorf("cannot extract the followed user id from a non-follow metadata")
	}
	return hex.EncodeToString(metadata.Data), nil
}

func CreateFollowUserMetadata(userID string, targetUserID string, unfollow bool) ContentMetadata {
	t := FOLLOW
	if unfollow {
		t = UNFOLLOW
	}
	data, _ := hex.DecodeString(targetUserID)
	return ContentMetadata{
		FeedUserID: userID,
		Type:       t,
		ContentID:  "",
		Timestamp:  utils.Time(),
		Data:       data,
		Signature:  nil,
	}
}

// ExtractEndorsedUserID extracts the target endorsed user id string from a ENDORSEMENT metadata object.
func ExtractEndorsedUserID(metadata ContentMetadata) (string, error) {
	if metadata.Type != ENDORSEMENT {
		return "", fmt.Errorf("cannot extract the endorsed user id from non-endorsement metadata")
	}
	return hex.EncodeToString(metadata.Data), nil
}

func CreateEndorseUserMetadata(userID string, targetUserID string) ContentMetadata {
	data, _ := hex.DecodeString(targetUserID)
	return ContentMetadata{
		FeedUserID: userID,
		Type:       ENDORSEMENT,
		ContentID:  "",
		Timestamp:  utils.Time(),
		Data:       data,
		Signature:  nil,
	}
}

func CreateEndorsementRequestMetadata(userID string) ContentMetadata {
	return ContentMetadata{
		FeedUserID: userID,
		Type:       ENDORSEMENT_REQUEST,
		ContentID:  "",
		Timestamp:  utils.Time(),
		Data:       nil,
		Signature:  nil,
	}
}
