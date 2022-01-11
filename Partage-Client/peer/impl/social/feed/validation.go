package feed

import (
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
	"go.dedis.ch/cs438/storage"
)

func (feedStore *Store) IsValidMetadata(c content.Metadata, blockchainStorage storage.MultipurposeStorage, metadataStore storage.Store) bool {
	// Accept reactions only when the user has not reacted to the referred content id yet.
	if c.Type == content.REACTION {
		return !feedStore.reactionHandler.AlreadyReacted(c.RefContentID, c.FeedUserID)
	}
	proposerFeed := feedStore.GetFeedCopy(blockchainStorage, metadataStore, c.FeedUserID)
	// Reject unknown users.
	if proposerFeed == nil {
		return false
	}
	// Make sure that the user can afford the cost.
	if c.Type.Cost() > proposerFeed.userState.CurrentCredits {
		return false
	}
	// Only process an endorsement request if the user's credit is lower than the set amount.
	if c.Type == content.ENDORSEMENT_REQUEST {
		withinRange := proposerFeed.userState.CurrentCredits <= ENDORSEMENT_REQUEST_CREDIT_LIMIT
		return withinRange && proposerFeed.userState.EndorsementHandler.CanRequest()
	}
	// Reject double-follows.
	if c.Type == content.FOLLOW {
		targetUserID, _ := content.ParseFollowedUser(c)
		return !proposerFeed.userState.IsFollowing(targetUserID)
	}
	// Check whether the given endorsement is valid.
	if c.Type == content.ENDORSEMENT {
		referredUser, _ := content.ParseEndorsedUserID(c)
		referredFeed := feedStore.GetFeedCopy(blockchainStorage, metadataStore, referredUser)
		return referredFeed.userState.CanEndorse(utils.Time(), c.FeedUserID)
	}
	// Check whether the attempted undo is valid.
	if c.Type == content.UNDO {
		referredHash, _ := content.ParseUndoMetadata(c)
		referredMetadata, err := proposerFeed.GetWithHash(referredHash)
		// The referred hash must exist.
		if err != nil {
			return false
		}
		// Referred metadata must be owned by the same user
		if c.FeedUserID != referredMetadata.FeedUserID {
			return false
		}
		// Only reactions, text, comments, and follows can be undone.
		referredMetadataIsUndoable := referredMetadata.Type == content.REACTION ||
			referredMetadata.Type == content.TEXT ||
			referredMetadata.Type == content.COMMENT ||
			referredMetadata.Type == content.FOLLOW
		return referredMetadataIsUndoable
	}
	return true
}
