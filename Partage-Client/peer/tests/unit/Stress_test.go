package unit

import (
	"fmt"
	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/peer/impl/content"
	"go.dedis.ch/cs438/peer/impl/utils"
	"sync"
	"testing"
	"time"
)

func CreateFullyConnectedSystem(t *testing.T, systemSize int) []z.TestNode {
	var nodes []z.TestNode
	// Create the nodes.
	fmt.Printf("\tCreateFullyConnectedSystem(%d): Creating the nodes...\n", systemSize)
	for i := 0; i < systemSize; i++ {
		n := z.NewTestNode(t, peerFac, tcpFac(), "127.0.0.1:0",
			z.WithTotalPeers(uint(systemSize)),
			z.WithPaxosID(uint(i+1)),
			z.WithAntiEntropy(time.Second),
		)
		nodes = append(nodes, n)
	}
	fmt.Printf("\tCreateFullyConnectedSystem(%d): Populating routing tables...\n", systemSize)
	// Populate the routing tables of each node.
	for i := 0; i < systemSize; i++ {
		n_i := nodes[i]
		for j := 0; j < systemSize; j++ {
			if i == j {
				continue
			}
			n_j := nodes[j]
			n_i.AddPeer(n_j.GetAddr())
		}
	}
	// Register the users.
	for i, n := range nodes {
		fmt.Printf("\tCreateFullyConnectedSystem(%d): Registering node %d...\n", systemSize, i)
		err := n.RegisterUser()
		require.Nil(t, err)
		time.Sleep(1 * time.Second)
	}
	return nodes
}

func Test_Partage_Global_Feed_Compare(t *testing.T) {
	// First, test with the global feed.
	for _, globalFeedSetting := range []bool{false, true} {
		utils.GLOBAL_FEED = globalFeedSetting
		systemSize := 17
		sizeIncrement := 1
		maxSize := 32
		for systemSize <= maxSize {
			fmt.Printf("System size = %d\n", systemSize)
			nodes := CreateFullyConnectedSystem(t, systemSize)
			// Start the timer.
			tStart := time.Now()
			// Propose a block.
			var wg sync.WaitGroup
			for _, n := range nodes {
				wg.Add(1)
				go func(n z.TestNode) {
					_, err := n.UpdateFeed(content.CreateChangeUsernameMetadata(n.GetUserID(), "tester"))
					require.Nil(t, err)
					wg.Done()
				}(n)
			}
			wg.Wait()
			// End the timer.
			tEnd := time.Now()
			// Measure the time.
			tDiff := tEnd.Sub(tStart)
			// Measure the # packets.
			numPackets := 0
			for _, n := range nodes {
				numPackets += len(n.GetOuts())
			}
			fmt.Printf("GlobalFeed = %v, Size = %d, t = %dms packets = %d\n", globalFeedSetting, systemSize, tDiff.Milliseconds(), numPackets)
			// Stop the nodes.
			for _, n := range nodes {
				err := n.Stop()
				require.Nil(t, err)
			}
			systemSize += sizeIncrement
		}
	}
}

func Test_Partage_Stress(t *testing.T) {
	systemSize := 2
	maxSize := 11
	for systemSize <= maxSize {
		fmt.Printf("System size = %d\n", systemSize)
		nodes := CreateFullyConnectedSystem(t, systemSize)
		// Start the timer.
		tStart := time.Now()
		// Propose a block.
		_, err := nodes[0].UpdateFeed(content.CreateChangeUsernameMetadata(nodes[0].GetUserID(), "tester"))
		require.Nil(t, err)
		// End the timer.
		tEnd := time.Now()
		// Measure the time.
		tDiff := tEnd.Sub(tStart)
		// Measure the # packets.
		numPackets := 0
		for _, n := range nodes {
			numPackets += len(n.GetOuts())
		}
		fmt.Printf("Size = %d, t = %dms packets=%d\n", systemSize, tDiff.Milliseconds(), numPackets)
		// Stop the nodes.
		for _, n := range nodes {
			err := n.Stop()
			require.Nil(t, err)
		}
		systemSize = systemSize + 5
	}
}
