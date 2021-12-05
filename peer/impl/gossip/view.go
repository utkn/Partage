package gossip

import (
	"sync"

	"go.dedis.ch/cs438/types"
)

type SeqMap map[string]uint
type RumorMap map[string]map[uint]types.Rumor

type PeerView struct {
	rumorMap     RumorMap
	rumorMapLock sync.Mutex
}

func NewPeerView() *PeerView {
	rm := make(RumorMap)
	return &PeerView{
		rumorMap: rm,
	}
}

func (v *PeerView) getSequence(peerAddr string) uint {
	rumorList, ok := v.rumorMap[peerAddr]
	if !ok {
		return 0
	}
	return uint(len(rumorList))
}

func (v *PeerView) IsExpected(peerAddr string, givenSequence uint) bool {
	v.rumorMapLock.Lock()
	defer v.rumorMapLock.Unlock()
	currSeq := v.getSequence(peerAddr)
	return currSeq+1 == givenSequence
}

func (v *PeerView) AsStatusMsg() types.StatusMessage {
	v.rumorMapLock.Lock()
	defer v.rumorMapLock.Unlock()
	statusMsg := types.StatusMessage{}
	for k := range v.rumorMap {
		statusMsg[k] = v.getSequence(k)
	}
	return statusMsg
}

func (v *PeerView) AsSequenceMap() SeqMap {
	v.rumorMapLock.Lock()
	defer v.rumorMapLock.Unlock()
	seqMap := make(SeqMap)
	for k := range v.rumorMap {
		seqMap[k] = v.getSequence(k)
	}
	return seqMap
}

func (v *PeerView) GetSequence(peerAddr string) uint {
	v.rumorMapLock.Lock()
	defer v.rumorMapLock.Unlock()
	return v.getSequence(peerAddr)
}

func (v *PeerView) SaveRumor(rumor types.Rumor) {
	v.rumorMapLock.Lock()
	defer v.rumorMapLock.Unlock()
	// Save the rumor into the table.
	rumors, ok := v.rumorMap[rumor.Origin]
	if !ok {
		v.rumorMap[rumor.Origin] = make(map[uint]types.Rumor)
		rumors = v.rumorMap[rumor.Origin]
	}
	rumors[rumor.Sequence] = rumor
}

func (v *PeerView) GetSavedRumor(origin string, sequence uint) (types.Rumor, bool) {
	v.rumorMapLock.Lock()
	defer v.rumorMapLock.Unlock()
	rumors, ok := v.rumorMap[origin]
	if !ok {
		return types.Rumor{}, false
	}
	rumor, ok := rumors[sequence]
	if !ok {
		return types.Rumor{}, false
	}
	return rumor, true
}

// Compare compares two views and returns a sequence map of the differences.
// First returned sequence map contains the new entries that remote peer has.
// Second returned sequence map contains the new entries that this peer has.
func (v *PeerView) Compare(remoteSequenceMap SeqMap) (SeqMap, SeqMap) {
	mySeqMap := v.AsSequenceMap()
	rmtNews := make(SeqMap)
	thsNews := make(SeqMap)
	// First find all the new peers that the remote view has, i.e., Remote - This
	for remotePeer, remoteSeq := range remoteSequenceMap {
		thisSeq, ok := mySeqMap[remotePeer]
		if !ok {
			thisSeq = 0
		}
		if thisSeq < remoteSeq {
			rmtNews[remotePeer] = remoteSeq
		}
	}
	// Then, find all the new peers that current view has, i.e., This - Remote
	for thisPeer, thisSeq := range mySeqMap {
		remoteSeq, ok := remoteSequenceMap[thisPeer]
		if !ok || thisSeq > remoteSeq {
			thsNews[thisPeer] = thisSeq
		}
	}
	// Return the two difference maps.
	return rmtNews, thsNews
}
