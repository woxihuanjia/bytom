package netsync

import (
	"sync"

	"github.com/bytom/protocol"
	"github.com/bytom/protocol/bc/legacy"
)

type PeerManager struct {
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

// NewProtocolManager returns a new ethereum sub protocol manager. The Ethereum sub protocol manages peers capable
// with the ethereum network.
func NewPeerManager(txpool *protocol.TxPool) (*PeerManager, error) {
	// Create the protocol manager with the base fields
	manager := &PeerManager{
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

func (pm *PeerManager) Start(maxPeers int) {
	pm.maxPeers = maxPeers

	// broadcast transactions
	// pm.txCh = make(chan core.TxPreEvent, txChanSize)
	// pm.txSub = pm.txpool.SubscribeTxPreEvent(pm.txCh)
	go pm.txBroadcastLoop()

	// // broadcast mined blocks
	// pm.minedBlockSub = pm.eventMux.Subscribe(core.NewMinedBlockEvent{})
	// go pm.minedBroadcastLoop()

	// // start sync handlers
	// go pm.syncer()
	// go pm.txsyncLoop()
}

func (self *PeerManager) txBroadcastLoop() {
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
func (pm *PeerManager) BroadcastTx(tx *legacy.Tx) {
	// Broadcast transaction to a batch of peers not knowing about it
	peers := pm.peers.PeersWithoutTx(tx.ID.Byte32())
	//FIXME include this again: peers = peers[:int(math.Sqrt(float64(len(peers))))]
	for _, peer := range peers {
		peer.SendTransaction(tx)
	}
	// log.Trace("Broadcast transaction", "hash", hash, "recipients", len(peers))
}
