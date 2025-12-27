package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	var messages []int
	var mu sync.Mutex
	seen := make(map[int]bool)

	n.Handle("broadcast", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		val := int(body["message"].(float64))

		mu.Lock()
		if seen[val] {
			mu.Unlock()
			return n.Reply(msg, map[string]string{"type": "broadcast_ok"})
		}

		seen[val] = true
		messages = append(messages, val)
		mu.Unlock()

		myID := n.ID()
		root := "n0"
		allNodes := n.NodeIDs()

		// Achieved 177 ms median latency, 237 ms max latency, and 22.78 msgs/op (~45 msgs/broadcast)
		if myID == root {
			for _, dest := range allNodes {
				if dest != myID && dest != msg.Src {
					go retryRPC(n, dest, body)
				}
			}
		} else if msg.Src != root {
			go retryRPC(n, root, body)
		}

		return n.Reply(msg, map[string]string{"type": "broadcast_ok"})
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		var body map[string]any

		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		mu.Lock()
		defer mu.Unlock()

		return n.Reply(msg, map[string]any{"type": "read_ok", "messages": messages})
	})

	n.Handle("topology", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		mu.Lock()
		defer mu.Unlock()

		return n.Reply(msg, map[string]string{"type": "topology_ok"})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

func retryRPC(n *maelstrom.Node, dest string, body map[string]any) {
	for {
		done := make(chan struct{})
		err := n.RPC(dest, body, func(reply maelstrom.Message) error {
			close(done)
			return nil
		})

		if err == nil {
			select {
			case <-done:
				return
			case <-time.After(500 * time.Millisecond):
			}
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}
}
