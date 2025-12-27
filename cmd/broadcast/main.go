package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type BatchBody struct {
	Type     string `json:"type"`
	Messages []int  `json:"messages"`
}

// Part 1: 177 ms median latency, 237 ms max latency, 22.78 msgs/op (~45 msgs/broadcast)
// Part 2: 269 ms median latency, 757 ms max latency, 2.80 msgs/op (~5.6 msgs/broadcast)

func main() {
	n := maelstrom.NewNode()
	var messages []int
	var pending []int
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
		pending = append(pending, val)
		mu.Unlock()

		return n.Reply(msg, map[string]string{"type": "broadcast_ok"})
	})

	n.Handle("batch_broadcast", func(msg maelstrom.Message) error {
		var body BatchBody
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		mu.Lock()
		for _, v := range body.Messages {
			if !seen[v] {
				seen[v] = true
				messages = append(messages, v)
				if n.ID() == "n0" {
					pending = append(pending, v)
				}
			}
		}
		mu.Unlock()

		return n.Reply(msg, map[string]string{"type": "batch_broadcast_ok"})
	})

	go func() {
		ticker := time.NewTicker(300 * time.Millisecond)
		for range ticker.C {
			mu.Lock()
			if len(pending) == 0 {
				mu.Unlock()
				continue
			}
			batchToSend := pending
			pending = []int{}
			mu.Unlock()

			myID := n.ID()
			root := "n0"
			payload := BatchBody{
				Type:     "batch_broadcast",
				Messages: batchToSend,
			}

			if myID == root {
				for _, dest := range n.NodeIDs() {
					if dest != myID {
						go retryRPC(n, dest, payload)
					}
				}
			} else {
				go retryRPC(n, root, payload)
			}
		}
	}()

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

func retryRPC(n *maelstrom.Node, dest string, body BatchBody) {
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
