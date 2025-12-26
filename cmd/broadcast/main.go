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
	topology := make(map[string]any)
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

		var neighbours []any
		if peers, ok := topology[n.ID()].([]any); ok {
			neighbours = peers
		}

		mu.Unlock()

		for _, nID := range neighbours {
			dest := nID.(string)
			if dest != msg.Src {
				go func(target string, messageBody map[string]any) {
					for {
						done := make(chan struct{})

						err := n.RPC(target, messageBody, func(reply maelstrom.Message) error {
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
				}(dest, body)
			}
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
		topology = body["topology"].(map[string]any)
		mu.Unlock()

		return n.Reply(msg, map[string]string{"type": "topology_ok"})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
