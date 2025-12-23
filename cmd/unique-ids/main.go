package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

/*
- Initial thought is to use a set to store the IDs, which is very consistent but doesn't address partition problem and is not efficient.
- Sticking to just the node, we can bundle unique information together, i.e. combine node ID, server ID, and message ID to get a unique ID.
[However, message IDs given by server IDs could be matching within a node, hence node ID-message ID itself isn't sufficient]
- We can either use a triple combo of the IDs since its all unique OR create a counter for each node ID:
- We choose the latter because:
   - Using internal counter allows node to remain sovereign and not dependent on a perfectly functioning client
   - Smaller IDs with the counter save bandwidth and memory
*/

func main() {
	n := maelstrom.NewNode()
	var counter int64

	n.Handle("generate", func(msg maelstrom.Message) error {
		newID := atomic.AddInt64(&counter, 1)
		var body map[string]any

		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		body["type"] = "generate_ok"
		body["id"] = fmt.Sprintf("%s-%d", n.ID(), newID)

		return n.Reply(msg, body)
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
