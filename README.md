# Maelstrom Distributed Systems Challenges

This repository contains my solutions to the [Gossip Glomers](https://fly.io/dist-sys/) distributed systems challenges using **Go** and the **Maelstrom** testing runtime.

## Project Structure

The project uses a standard Go `cmd` layout to manage multiple binaries within a single module.

* `cmd/echo/`: Source code for Challenge #1.
* `cmd/unique-ids/`: Source code for Challenge #2.
* `cmd/broadcast/`: Source code for Challenge #3.
* `cmd/grow-only-counter/`: Source code for Challenge #4.
* `cmd/kafka/`: Source code for Challenge #5.
* `cmd/kv-store/`: Source code for Challenge #6. 
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

### 4. Grow-Only Counter
**Goal:** Implement a stateless, grow-only counter that maintains a consistent global sum across a distributed cluster.
* **Key Learnings:**
    * Each node is responsible for its own counter, stored in the KV store using the node's unique ID as the key.
    * Increments are performed using a Read-Modify-Write loop protected by Compare-And-Swap method to prevent lost updates when multiple requests hit the same node.
    * The total count is calculated by iterating through all known node IDs, reading their individual buckets, and summing them locally.
    * For eventual consistency, added a small delay (50ms) in the read handler to ensure that after a network partition "heals," the node allows enough time for the latest KV writes to propagate before returning the final sum.
    * Specifically handles KeyDoesNotExist errors during reads by defaulting the bucket value to 0, allowing the counter to initialize correctly.
    * We ensure statelessness as the Go binary holds no local counter state in RAM, ensuring that if a node restarts, it can resume counting immediately from the KV store.
* **Build:** `go build -o maelstrom-counter ./cmd/grow-only-counter`.

### 5. Kafka-Style Log
**Goal:** Implement a replicated log service that supports multiple named logs (keys), offset-based consumption, and offset committing.
* **Key Learnings:**
    * This implementation treats the Linearizable Key-Value Store (lin-kv) as the single source of truth. Every node acts as a stateless proxy that coordinates between the client and the KV store.
    * Some bottlenecks include the network amplification phenomenon where the `send` handler is constantly transmitting the whole history as more messages are sent and Compare-And-Swap failures if we increase the number of nodes competing to update the log at the same time.
    * Deep Dive of Methods in Efficiency Exploration:
       * **Baseline**: This (implemented) approach stored the entire log as a single JSON array in `lin-kv`
          * Pros: Extremely simple to reason about; guaranteed linearizability via `CompareAndSwap`
          * Cons: Bottlenecks with `send` handler and node counts as discussed above.
       * **Tail Pointer**: Moving to a system where we CAS a single "tail" integer and store messages at unique keys
          * Pros: Drastically reduced payload size per request.
          * Cons: Exploded the number of RPCs. A `poll` for 100 messages now required 100 individual `kv.Read` calls, overwhelming the Maelstrom network limits.
       * **Key Sharding**: Assigning specific keys to specific nodes to eliminate cross-node contention.
          * Pros: Only one node writes to a specific key, virtually eliminating CAS failures.
          * Cons: Requires proxying. If the destination node is busy, proxying adds hop-latency and increases total message count without necessarily improving throughput.
       * **Batching**: Utilizing a Go channel as a buffer and a background worker to "flush" multiple messages in a single KV transaction.
          * Pros: By "lingering" for 50ms, we can turn 10 send requests into 1 CAS call. 
          * Cons: A single user might see slower response times even though the system's total capacity is much higher.
* **Results:** (with Baseline)
   * Entire network cost of 7.19 messages/operation and server cost (internal traffic between nodes and KV store) of **4.73 msgs/op**
   * 99.95% availability
   * Satisfied no lost writes and monotonic offsets (accomplished with lin-kv in place of seq-kv; lin-kv always sees latest and is safe for CAS operations at the expensve of being slower, meanwhile sequential may see stale data and risk offset collisions but it is faster since it is merely going for "eventual consistency")
 
### 6. Key-Value Store
**Goal:** Implement a totally available, transactional key-value store that supports multi-key transactions with varying levels of isolation (Read Uncommitted and Read Committed).
* **Key Learnings:**
    * For single node, totally available, we used a sync.RWMutex to ensure atomic execution of a list of operations.
    * For multi node, totally available (read uncommitted), we introduced background replication where nodes periodically broadcast their local kv snapshots to neighbors.
      * Pros: Guaranteed availability; eventually all nodes see all unique writes.
      * Cons: Allows Dirty Reads; nodes might see uncommitted data from other transactions.
      * G0 Prevention: Since writes are unique per-key in this workload, merging incoming gossip into a local map naturally prevents "Dirty Writes" (clobbering data).
    * For multi node, totally available (read committed), implemented a local buffer inside the transaction handler.
      * Pros: Prevents G1a/b/c anomalies (Aborted Reads, Intermediate Reads). Other nodes only see a transaction's effect once it is fully committed locally.
      * Cons: Increased memory overhead per transaction to hold the buffer before the final "publish" to the global state.

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

# Challenge 5: Kafka-Style Log
./maelstrom test -w kafka --bin ./maelstrom-kafka --node-count 2 --concurrency 2n --time-limit 20 --rate 1000

# Challenge 6: Key-Value Store
./maelstrom test -w txn-rw-register --bin ~/go/bin/maelstrom-txn --node-count 1 --time-limit 20 --rate 1000 --concurrency 2n --consistency-models read-uncommitted --availability total
