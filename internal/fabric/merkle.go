// Distributed MemOS - Fabric: Merkle Tree for Anti-Entropy
package fabric

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// MerkleNode represents a node in the Merkle Tree.
type MerkleNode struct {
	Hash  string
	Left  *MerkleNode
	Right *MerkleNode
}

// MerkleTree represents the entire tree for a memory shard.
type MerkleTree struct {
	Root *MerkleNode
}

// NewMerkleNode creates a new node with hash of data or combined hashes of children.
func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	node := &MerkleNode{}
	hash := sha256.New()

	if left == nil && right == nil {
		hash.Write(data)
	} else {
		prevHashes := left.Hash
		if right != nil {
			prevHashes += right.Hash
		}
		hash.Write([]byte(prevHashes))
	}

	node.Hash = hex.EncodeToString(hash.Sum(nil))
	node.Left = left
	node.Right = right
	return node
}

// BuildMerkleTree constructs a Merkle Tree from a list of memory IDs and their content hashes.
func BuildMerkleTree(data [][]byte) *MerkleTree {
	if len(data) == 0 {
		return &MerkleTree{Root: &MerkleNode{Hash: hex.EncodeToString(make([]byte, 32))}}
	}

	var nodes []*MerkleNode
	for _, d := range data {
		nodes = append(nodes, NewMerkleNode(nil, nil, d))
	}

	// Sort nodes by hash for deterministic tree generation
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Hash < nodes[j].Hash
	})

	for len(nodes) > 1 {
		var nextLevel []*MerkleNode
		for i := 0; i < len(nodes); i += 2 {
			if i+1 < len(nodes) {
				nextLevel = append(nextLevel, NewMerkleNode(nodes[i], nodes[i+1], nil))
			} else {
				nextLevel = append(nextLevel, NewMerkleNode(nodes[i], nil, nil))
			}
		}
		nodes = nextLevel
	}

	return &MerkleTree{Root: nodes[0]}
}

// GetRootHash returns the hex string of the root.
func (t *MerkleTree) GetRootHash() string {
	if t.Root == nil {
		return ""
	}
	return t.Root.Hash
}
