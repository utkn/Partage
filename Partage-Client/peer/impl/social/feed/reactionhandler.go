package feed

import (
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
	"sync"
)

type ReactionHandler struct {
	sync.RWMutex
	// Maps a content id to all of its reactions.
	reactionMap map[string][]content.ReactionInfo
}

func NewReactionHandler() *ReactionHandler {
	return &ReactionHandler{
		reactionMap: make(map[string][]content.ReactionInfo),
	}
}

// AlreadyReacted returns true if the given user has reacted to the given content.
func (h *ReactionHandler) AlreadyReacted(contentID string, userID string) bool {
	h.RLock()
	defer h.RUnlock()
	// If already reacted, do not save!
	for _, reactionInfo := range h.reactionMap[contentID] {
		if reactionInfo.UserID == userID {
			utils.PrintDebug("social", "re-react not allowed")
			return true
		}
	}
	return false
}

// SaveReaction tries to save the given reaction.
func (h *ReactionHandler) SaveReaction(reaction content.Reaction, contentID string, userID string) {
	// Do not want to save reactions if already reacted.
	if h.AlreadyReacted(contentID, userID) {
		return
	}
	h.Lock()
	defer h.Unlock()
	_, ok := h.reactionMap[contentID]
	if !ok {
		h.reactionMap[contentID] = []content.ReactionInfo{}
	}
	// Save.
	reactionInfo := content.ReactionInfo{
		Reaction:     reaction,
		UserID:       userID,
		RefContentID: contentID,
	}
	h.reactionMap[contentID] = append(h.reactionMap[contentID], reactionInfo)
}

// UndoReaction tries to remove the reaction made by the given user from the given content id.
func (h *ReactionHandler) UndoReaction(contentID string, userID string) {
	h.Lock()
	defer h.Unlock()
	reactions, ok := h.reactionMap[contentID]
	if !ok {
		return
	}
	indexToRemove := -1
	for i, r := range reactions {
		if r.UserID == userID {
			indexToRemove = i
			break
		}
	}
	if indexToRemove < 0 {
		return
	}
	// Delete the associated reaction.
	h.reactionMap[contentID] = append(reactions[:indexToRemove], reactions[indexToRemove+1:]...)
}

func (h *ReactionHandler) GetReactionsCopy(contentID string) []content.ReactionInfo {
	h.RLock()
	reactions, _ := h.reactionMap[contentID]
	h.RUnlock()
	reactionsCopied := make([]content.ReactionInfo, 0, len(reactions))
	for _, r := range reactions {
		reactionsCopied = append(reactionsCopied, r)
	}
	return reactionsCopied
}
