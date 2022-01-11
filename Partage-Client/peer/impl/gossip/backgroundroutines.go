package gossip

import (
	"fmt"
	"go.dedis.ch/cs438/peer/impl/utils"
	"time"

	"go.dedis.ch/cs438/types"
)

func AntiEntropy(n *Layer, interval time.Duration) {
	quitListener, _ := n.quitDistributor.GetListener("antientropy")
	for {
		select {
		case <-quitListener:
			utils.PrintDebug("antientropy", n.GetAddress(), "quitting anti-entropy")
			return
		default:
			utils.PrintDebug("antientropy", n.GetAddress(), "has initiated anti-entropy")
			time.Sleep(interval)
			statusMsg := n.view.AsStatusMsg()
			if n.cryptography!=nil{
				for _,ip:=range(n.cryptography.GetBlockedIPs()){
					statusMsg[ip]=-1
				}
			}
			dest, err := n.network.ChooseRandomNeighbor(nil)
			if err != nil {
				continue
			}
			transpMsg, err := n.config.MessageRegistry.MarshalMessage(&statusMsg)
			if err != nil {
				fmt.Println("error during anti-entropy:", err)
				break
			}
			if n.cryptography != nil {
				n.cryptography.Unicast(dest, transpMsg)
			} else {
				n.network.Unicast(dest, transpMsg)
			}
		}
	}
}

func Heartbeat(n *Layer, interval time.Duration) {
	quitListener, _ := n.quitDistributor.GetListener("heartbeat")
	for {
		select {
		case <-quitListener:
			utils.PrintDebug("heartbeat", n.GetAddress(), "quitting heartbeat")
			return
		default:
			utils.PrintDebug("heartbeat", n.GetAddress(), "has started heartbeat")
			emptyMsg := types.EmptyMessage{}
			err := n.BroadcastMessage(emptyMsg)
			if err != nil {
				fmt.Println("Error during heartbeat", err)
			}
			time.Sleep(interval)
		}
	}
}
