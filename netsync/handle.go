package netsync

import (
	"math"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytom/common"
	"github.com/bytom/netsync/downloader"
	"github.com/bytom/protocol"
	"github.com/bytom/protocol/bc/legacy"
	"github.com/ethereum/go-ethereum/core"
	log "github.com/sirupsen/logrus"
	"github.com/tendermint/tendermint/types"
)

type SyncManager struct {
	networkId uint64

	// fastSync  uint32 // Flag whether fast sync is enabled (gets disabled if we already have blocks)
	// acceptTxs uint32 // Flag whether we're considered synchronised (enables transaction processing)
	txpool *protocol.TxPool
	// txpool      txPool
	// blockchain  *core.BlockChain
	// chainconfig *params.ChainConfig
	maxPeers int

	// downloader *downloader.Downloader
	// fetcher    *fetcher.Fetcher
	peers *peerSet

	// SubProtocols []p2p.Protocol

	// eventMux      *event.TypeMux
	// txCh          chan core.TxPreEvent
	// txSub         event.Subscription
	// minedBlockSub *event.TypeMuxSubscription

	// channels for fetcher, syncer, txsyncLoop
	newPeerCh chan *peer
	// txsyncCh    chan *txsync
	quitSync    chan struct{}
	noMorePeers chan struct{}

	// wait group is used for graceful shutdowns during downloading
	// and processing
	wg sync.WaitGroup
}

// NewSyncManager returns a new ethereum sub protocol manager. The Ethereum sub protocol manages peers capable
// with the ethereum network.
func NewSyncManager(txpool *protocol.TxPool) (*SyncManager, error) {
	// Create the protocol manager with the base fields
	manager := &SyncManager{
		// networkId: networkId,
		// eventMux:    mux,
		txpool: txpool,
		// blockchain:  blockchain,
		// chainconfig: config,
		peers:       newPeerSet(),
		newPeerCh:   make(chan *peer),
		noMorePeers: make(chan struct{}),
		// txsyncCh:    make(chan *txsync),
		quitSync: make(chan struct{}),
	}

	return manager, nil
}

func (self *SyncManager) Start(maxPeers int) {
	self.maxPeers = maxPeers

	// broadcast transactions
	// self.txCh = make(chan core.TxPreEvent, txChanSize)
	// self.txSub = self.txpool.SubscribeTxPreEvent(self.txCh)
	go self.txBroadcastLoop()

	// // broadcast mined blocks
	// self.minedBlockSub = self.eventMux.Subscribe(core.NewMinedBlockEvent{})
	go self.minedBroadcastLoop()

	// // start sync handlers
	go self.syncer()
	// go self.txsyncLoop()
}

func (self *SyncManager) txBroadcastLoop() {
	newTxCh := self.txpool.GetNewTxCh()

	for {
		select {
		case newTx := <-newTxCh:
			self.BroadcastTx(newTx)

		// // Err() channel will be closed when unsubscribing.
		// case <-self.txSub.Err():
		// 	return
		case <-self.quitSync:
			return
		}
	}
}

// BroadcastTx will propagate a transaction to all peers which are not known to
// already have the given transaction.
func (self *SyncManager) BroadcastTx(tx *legacy.Tx) {
	// Broadcast transaction to a batch of peers not knowing about it
	peers := self.peers.PeersWithoutTx(tx.ID.Byte32())
	//FIXME include this again: peers = peers[:int(math.Sqrt(float64(len(peers))))]
	for _, peer := range peers {
		peer.SendTransaction(tx)
	}
	// log.Trace("Broadcast transaction", "hash", hash, "recipients", len(peers))
}

// Mined broadcast loop
func (self *SyncManager) minedBroadcastLoop() {
	// automatically stops if unsubscribe
	for obj := range self.minedBlockSub.Chan() {
		switch ev := obj.Data.(type) {
		case core.NewMinedBlockEvent:
			self.BroadcastBlock(ev.Block, true)  // First propagate block to peers
			self.BroadcastBlock(ev.Block, false) // Only then announce to the rest
		}
	}
}

// BroadcastBlock will either propagate a block to a subset of it's peers, or
// will only announce it's availability (depending what's requested).
func (self *SyncManager) BroadcastBlock(block *types.Block, propagate bool) {
	hash := block.Hash()
	peers := self.peers.PeersWithoutBlock(hash)

	// If propagation is requested, send to a subset of the peer
	if propagate {
		// Calculate the TD of the block (it's not imported yet, so block.Td is not valid)
		var td *big.Int
		if parent := self.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1); parent != nil {
			td = new(big.Int).Add(block.Difficulty(), self.blockchain.GetTd(block.ParentHash(), block.NumberU64()-1))
		} else {
			log.Error("Propagating dangling block", "number", block.Number(), "hash", hash)
			return
		}
		// Send the block to a subset of our peers
		transfer := peers[:int(math.Sqrt(float64(len(peers))))]
		for _, peer := range transfer {
			peer.SendNewBlock(block, td)
		}
		log.Trace("Propagated block", "hash", hash, "recipients", len(transfer), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))
		return
	}
	// Otherwise if the block is indeed in out own chain, announce it
	if self.blockchain.HasBlock(hash, block.NumberU64()) {
		for _, peer := range peers {
			peer.SendNewBlockHashes([]common.Hash{hash}, []uint64{block.NumberU64()})
		}
		log.Trace("Announced block", "hash", hash, "recipients", len(peers), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))
	}
}

// syncer is responsible for periodically synchronising with the network, both
// downloading hashes and blocks as well as handling the announcement handler.
func (self *SyncManager) syncer() {
	// Start and ensure cleanup of sync mechanisms
	self.fetcher.Start()
	defer self.fetcher.Stop()
	defer self.downloader.Terminate()

	// Wait for different events to fire synchronisation operations
	forceSync := time.NewTicker(forceSyncCycle)
	defer forceSync.Stop()

	for {
		select {
		case <-self.newPeerCh:
			// Make sure we have peers to select from, then sync
			if self.peers.Len() < minDesiredPeerCount {
				break
			}
			go self.synchronise(self.peers.BestPeer())

		case <-forceSync.C:
			// Force a sync even if not enough peers are present
			go self.synchronise(self.peers.BestPeer())

		case <-self.noMorePeers:
			return
		}
	}
}

// synchronise tries to sync up our local block chain with a remote peer.
func (self *SyncManager) synchronise(peer *peer) {
	// Short circuit if no peers are available
	if peer == nil {
		return
	}
	// Make sure the peer's TD is higher than our own
	currentBlock := self.blockchain.CurrentBlock()
	td := self.blockchain.GetTd(currentBlock.Hash(), currentBlock.NumberU64())

	pHead, pTd := peer.Head()
	if pTd.Cmp(td) <= 0 {
		return
	}
	// Otherwise try to sync with the downloader
	mode := downloader.FullSync
	if atomic.LoadUint32(&self.fastSync) == 1 {
		// Fast sync was explicitly requested, and explicitly granted
		mode = downloader.FastSync
	} else if currentBlock.NumberU64() == 0 && self.blockchain.CurrentFastBlock().NumberU64() > 0 {
		// The database seems empty as the current block is the genesis. Yet the fast
		// block is ahead, so fast sync was enabled for this node at a certain point.
		// The only scenario where this can happen is if the user manually (or via a
		// bad block) rolled back a fast sync node below the sync point. In this case
		// however it's safe to reenable fast sync.
		atomic.StoreUint32(&self.fastSync, 1)
		mode = downloader.FastSync
	}
	// Run the sync cycle, and disable fast sync if we've went past the pivot block
	if err := self.downloader.Synchronise(peer.id, pHead, pTd, mode); err != nil {
		return
	}
	if atomic.LoadUint32(&self.fastSync) == 1 {
		log.Info("Fast sync complete, auto disabling")
		atomic.StoreUint32(&self.fastSync, 0)
	}
	atomic.StoreUint32(&self.acceptTxs, 1) // Mark initial sync done
	if head := self.blockchain.CurrentBlock(); head.NumberU64() > 0 {
		// We've completed a sync cycle, notify all peers of new state. This path is
		// essential in star-topology networks where a gateway node needs to notify
		// all its out-of-date peers of the availability of a new block. This failure
		// scenario will most often crop up in private and hackathon networks with
		// degenerate connectivity, but it should be healthy for the mainnet too to
		// more reliably update peers or the local TD state.
		go self.BroadcastBlock(head, false)
	}
}
