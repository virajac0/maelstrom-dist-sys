# Maelstrom Distributed Systems Challenges

This repository contains my solutions to the [Gossip Glomers](https://fly.io/dist-sys/) distributed systems challenges using **Go** and the **Maelstrom** testing runtime.

## Project Structure

The project uses a standard Go `cmd` layout to manage multiple binaries within a single module.

* `cmd/echo/`: Source code for Challenge #1.
* `cmd/unique-ids/`: Source code for Challenge #2.
* `cmd/broadcast/`: Source code for Challenge #3.
* `cmd/grow-only-counter/`: Source code for Challenge #4.
* `go.mod`: Project dependencies and module definition (`maelstrom-challenges`).

## Completed Challenges

### 1. Echo
**Goal:** Implement a node that responds to "echo" messages with an "echo_ok" message.
* **Key Learnings:** Standard input/output protocol for Maelstrom and basic JSON message handling.
* **Build:** `go build -o maelstrom-echo ./cmd/echo`.

### 2. Unique ID Generation
**Goal:** Implement a globally unique ID generator that is **totally available**, even during network partitions.
* **Key Learnings:**
    * Implementing thread-safe counters in Go using the `sync/atomic` package to ensure concurrency safety.
    * The approach involves using a combination of the node ID and an atomic counter to ensure uniqueness without cross-node communication. This enables us to prioritize availability over consistency in the event of network partitions. 
* **Build:** `go build -o maelstrom-unique-ids ./cmd/unique-ids`.

### 3. Broadcast
**Goal:** Implement a distributed broadcast system where nodes gossip messages to reach eventual consistency across the cluster, even during network partitions.
* **Key Learnings:**
    * Single Node Broadcast: basic implementation of the three handlers (broadcast, read, and topology)
    * Multi Node Broadcast: using locks in the three handlers to ensure thread safety; in broadcast, we can use a "seen" map to efficiently identify if a message has been seen be a node instance, otherwise utilize a n.Send() call.
    * Fault Tolerant Broadcast: Retry Loop --> utilize a goroutine that involves a channel for a n.RPC() call with time after or sleep logic to continue retrying
    * Efficient Broadcast, Pt 1: optimize network topology from a 2D grid to a flat star topology using "n0" as a hub to all other nodes as leaves. This limits the number of hops to two and we can have a retry RPC in place for the root in case of failure.
    * Efficient Broadcast, Pt 2: achieve extreme message efficiency by using broadcast batching (trading small latency for higher throughput). Instead of nodes sending one message per value, they collect a bunch of values and then send (so either leaves collecting then sending slice of values to root to be sent out or the root sending to leaves a slice of values at once).
* **Results:**
    * Efficient Broadcast, Pt 1: 177 ms median latency, 237 ms max latency, 22.78 msgs/op (~45 msgs/broadcast)
    * Efficient Broadcast, Pt 2: 269 ms median latency, 757 ms max latency, 2.80 msgs/op (~5.6 msgs/broadcast)
* **Build:** `go build -o maelstrom-unique-ids ./cmd/broadcast`.

### 3. Grow-Only Counter
**Goal:** Implement a stateless, grow-only counter that maintains a consistent global sum across a distributed cluster.
* **Key Learnings:**
    * Each node is responsible for its own counter, stored in the KV store using the node's unique ID as the key.
    * Increments are performed using a Read-Modify-Write loop protected by Compare-And-Swap method to prevent lost updates when multiple requests hit the same node.
    * The total count is calculated by iterating through all known node IDs, reading their individual buckets, and summing them locally.
    * For eventual consistency, added a small delay (50ms) in the read handler to ensure that after a network partition "heals," the node allows enough time for the latest KV writes to propagate before returning the final sum.
    * Specifically handles KeyDoesNotExist errors during reads by defaulting the bucket value to 0, allowing the counter to initialize correctly.
    * We ensure statelessness as the Go binary holds no local counter state in RAM, ensuring that if a node restarts, it can resume counting immediately from the KV store.
* **Build:** `go build -o maelstrom-counter ./cmd/grow-only-counter`.
  
## Testing

To run the challenges, ensure you have the `maelstrom` binary installed and run the following commands from the root directory:

```bash
# Challenge 1: Echo
./maelstrom test -w echo --bin ./maelstrom-echo --node-count 1 --time-limit 10

# Challenge 2: Unique IDs
./maelstrom test -w unique-ids --bin ./maelstrom-unique-ids --time-limit 30 --rate 1000 --node-count 3 --availability total --nemesis partition

# Challenge 3: Broadcast
./maelstrom test -w broadcast --bin ./maelstrom-broadcast --node-count 25 --time-limit 20 --rate 100 --latency 100

# Challenge 4: Grow-Only Counter
./maelstrom test -w g-counter --bin ./maelstrom-counter --node-count 3 --rate 100 --time-limit 20 --nemesis partition
