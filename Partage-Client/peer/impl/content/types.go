package content

import (
	"encoding/hex"
	"fmt"
	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer/impl/utils"
	"strconv"
)

type Type int

const (
	TEXT Type = iota
	COMMENT
	REACTION
	FOLLOW
	USERNAME
	ENDORSEMENT
	ENDORSEMENT_REQUEST
	UNDO
)

func (c Type) String() string {
	switch c {
	case TEXT:
		return "text"
	case COMMENT:
		return "comment"
	case REACTION:
		return "reaction"
	case FOLLOW:
		return "follow"
	case USERNAME:
		return "username"
	case ENDORSEMENT:
		return "endorsement"
	case ENDORSEMENT_REQUEST:
		return "endorsement_request"
	case UNDO:
		return "undo"
	}
	return "unknown"
}

func (c Type) Cost() int {
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

func CreateFollowUserMetadata(userID string, targetUserID string) Metadata {
	data, _ := hex.DecodeString(targetUserID)
	return Metadata{
		FeedUserID: userID,
		Type:       FOLLOW,
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

func CreateTextMetadata(userID string, timestamp int64, metahash string) Metadata {
	// Create a random content id.
	contentID := xid.New().String()
	return Metadata{
		Type:       TEXT,
		ContentID:  contentID,
		FeedUserID: userID,
		Timestamp:  timestamp,
		Data:       []byte(metahash),
		Signature:  nil,
	}
}

// CreateCommentMetadata ...
// refContentID is the content id of the post that the comment is made for.
func CreateCommentMetadata(userID string, timestamp int64, refContentID string, metahash string) Metadata {
	// Create a random content id.
	contentID := xid.New().String()
	return Metadata{
		Type:         COMMENT,
		ContentID:    contentID,
		FeedUserID:   userID,
		RefContentID: refContentID,
		Timestamp:    timestamp,
		Data:         []byte(metahash),
		Signature:    nil,
	}
}

func CreateReactionMetadata(userID string, reaction Reaction, timestamp int64, refContentID string) Metadata {
	return Metadata{
		Type:         REACTION,
		FeedUserID:   userID,
		RefContentID: refContentID,
		Timestamp:    timestamp,
		Data:         []byte(strconv.Itoa(int(reaction))),
		Signature:    nil,
	}
}

func CreateUndoMetadata(userID string, timestamp int64, targetBlockHash string) Metadata {
	targetHashBytes, _ := hex.DecodeString(targetBlockHash)
	return Metadata{
		Type:       UNDO,
		FeedUserID: userID,
		Timestamp:  timestamp,
		Data:       targetHashBytes,
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

// ParseFollowedUser extracts the target followed user id string from a FOLLOW metadata object.
func ParseFollowedUser(metadata Metadata) (string, error) {
	if metadata.Type != FOLLOW {
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

// ParsePostMetadata extracts the metahash for the post object from a TEXT or COMMENT metadata object.
func ParsePostMetadata(metadata Metadata) (string, error) {
	if metadata.Type != TEXT && metadata.Type != COMMENT {
		return "", fmt.Errorf("cannot extract the metahash from non-text metadata")
	}
	return string(metadata.Data), nil
}

// ParseReactionMetadata extracts the reaction itself from the REACTION metadata object.
func ParseReactionMetadata(metadata Metadata) (Reaction, error) {
	if metadata.Type != REACTION {
		return ANGRY, fmt.Errorf("cannot extract the reaction from non-reaction metadata")
	}
	reactionInt, err := strconv.Atoi(string(metadata.Data))
	return Reaction(reactionInt), err
}

// ParseUndoMetadata extracts the referred block hash to undo from a UNDO metadata object.
func ParseUndoMetadata(metadata Metadata) (string, error) {
	if metadata.Type != UNDO {
		return "", fmt.Errorf("cannot extract the reaction from non-reaction metadata")
	}
	return hex.EncodeToString(metadata.Data), nil
}
