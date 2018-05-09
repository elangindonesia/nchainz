package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Node struct {
	myIp        string
	peers       map[string]*Peer
	bcs         *Blockchains
	mu          sync.RWMutex
	reconcileMu sync.Mutex
}

type Peer struct {
	ip      string
	state   PeerState
	client  *rpc.Client
	addedAt time.Time
}

type MsgType uint

const (
	VERSION MsgType = iota + 1
	GETBLOCKS
	INV
	GETDATA
	BLOCK
	TX
	ADDR
)

type PeerState uint

const (
	FOUND PeerState = iota + 1
	ACTIVE
	EXPIRED
	INVALID
	UNKNOWN
)

func StartNode(myIp string) {
	port, _ := strconv.Atoi(strings.Split(myIp, ":")[1])
	dbName := fmt.Sprintf("db/%v.db", port)
	localIp := fmt.Sprintf(":%v", port)

	addr, err := net.ResolveTCPAddr("tcp", localIp)
	if err != nil {
		LogFatal(err.Error())
	}

	inbound, err := net.ListenTCP("tcp", addr)
	if err != nil {
		LogFatal(err.Error())
	}

	peers := make(map[string]*Peer)
	bcs := CreateNewBlockchains(dbName, true)
	mu := sync.RWMutex{}
	reconcileMu := sync.Mutex{}
	node := &Node{myIp, peers, bcs, mu, reconcileMu}

	log.SetOutput(ioutil.Discard)
	rpc.Register(node)
	log.SetOutput(os.Stderr)

	seeds, err := GetSeeds()
	if err != nil {
		LogFatal("Couldn't find seeds file. Run \"echo SEED_IP:SEED_PORT > seeds.txt\" and try again.")
	}

	for _, seed := range seeds {
		go node.connectPeerIfNew(seed)
	}

	go node.invLoop()

	Log("Listening on %s", myIp)
	rpc.Accept(inbound)
}

//////////////////////////////////
// VERSION
// Handshake for a new peer
// request: data about yourself
// response: true if successful

type VersionArgs struct {
	Version      int
	StartHeights map[string]uint64
	From         string
}

func (node *Node) Version(args *VersionArgs, reply *bool) error {
	Log("Received VERSION from %s", args.From)
	defer Log("Done handling VERSION from %s", args.From)

	isNew, _, err := node.connectPeerIfNew(args.From)
	if !isNew {
		*reply = false
		return nil
	}
	if err != nil {
		*reply = false
		return err
	}

	*reply = true
	return nil
}

func (node *Node) SendVersion(peer *Peer) {
	version := 0
	startHeights := node.bcs.GetHeights()
	args := VersionArgs{version, startHeights, node.myIp}

	node.callVersion(peer, &args)
}

func (node *Node) callVersion(peer *Peer, args *VersionArgs) {
	Log("Sending VERSION to %s", peer.ip)
	defer Log("Done sending VERSION to %s", peer.ip)

	var reply bool
	err := peer.client.Call("Node.Version", &args, &reply)
	node.handleRpcReply(peer, err)
}

////////////////////////////////
// ADDR
// Share list of known peers
// request: peer list
// response: true if successful

type AddrArgs struct {
	Ips  []string
	From string
}

func (node *Node) Addr(args *AddrArgs, reply *bool) error {
	Log("Received ADDR from %s", args.From)
	defer Log("Done handling ADDR from %s", args.From)

	peerState := node.getPeerState(args.From)
	if peerState != ACTIVE && peerState != FOUND {
		Log("Received ADDR from inactive or unknown peer %s", args.From)
		*reply = false
		go node.connectPeerIfNew(args.From)
		return nil
	}

	for _, ip := range args.Ips {
		go node.connectPeerIfNew(ip)
	}

	*reply = true
	return nil
}

func (node *Node) SendAddr(peer *Peer) {
	args := node.getAddrArgs()
	node.callAddr(peer, args)
}

func (node *Node) BroadcastAddr() {
	args := node.getAddrArgs()

	for _, peer := range node.getActivePeers() {
		go node.callAddr(peer, args)
	}
}

func (node *Node) getAddrArgs() *AddrArgs {
	ips := node.getPeerIps()
	args := AddrArgs{ips, node.myIp}

	return &args
}

func (node *Node) callAddr(peer *Peer, args *AddrArgs) {
	Log("Sending ADDR to %s", peer.ip)
	defer Log("Done sending ADDR to %s", peer.ip)

	var reply bool
	err := peer.client.Call("Node.Addr", args, &reply)
	node.handleRpcReply(peer, err)
}

//////////////////////////////////////////
// INV
// Share blockhashes with peer
// request: all blockhashes for all chains
// response: true if successful

type InvArgs struct {
	Blockhashes  map[string][][]byte
	StartHeights map[string]uint64
	From         string
}

func (node *Node) Inv(args *InvArgs, reply *bool) error {
	Log("Received INV from %s", args.From)
	defer Log("Done handling INV from %s", args.From)

	peerState := node.getPeerState(args.From)
	if peerState != ACTIVE && peerState != FOUND {
		Log("Received INV from inactive or unknown peer %s", args.From)
		*reply = false
		go node.connectPeerIfNew(args.From)
		return nil
	}

	myHeights := node.bcs.GetHeights()

	for symbol, startHeight := range args.StartHeights {
		myHeight, ok := myHeights[symbol]

		if ok && myHeight < startHeight {
			go node.reconcileChain(args.From, symbol, args.Blockhashes[symbol], startHeight)
		}
	}

	*reply = true
	return nil
}

func (node *Node) SendInv(peer *Peer) {
	args := node.getInvArgs()
	node.callInv(peer, args)
}

func (node *Node) BroadcastInv() {
	args := node.getInvArgs()

	for _, peer := range node.getActivePeers() {
		go node.callInv(peer, args)
	}
}

func (node *Node) getInvArgs() *InvArgs {
	blockhashes := node.bcs.GetBlockhashes()
	startHeights := node.bcs.GetHeights()
	args := InvArgs{blockhashes, startHeights, node.myIp}

	return &args
}

func (node *Node) callInv(peer *Peer, args *InvArgs) {
	Log("Sending INV to %s", peer.ip)
	defer Log("Done sending INV to %s", peer.ip)

	var reply bool
	err := peer.client.Call("Node.Inv", args, &reply)
	node.handleRpcReply(peer, err)
}

func (node *Node) invLoop() {
	interval := 10 * time.Second
	ticker := time.NewTicker(interval)

	for {
		<-ticker.C
		node.BroadcastInv()
	}
}

/////////////////////////////////////////
// GETBLOCK
// Get a block on a chain
// request: blockhash of requested block
// response: the requested block

type GetBlockArgs struct {
	Blockhash []byte
	Symbol    string
	From      string
}

type GetBlockReply struct {
	Success bool
	Block   Block
}

func (node *Node) GetBlock(args *GetBlockArgs, reply *GetBlockReply) error {
	Log("Received GETBLOCK from %s for block %x on chain %s", args.From, args.Blockhash, args.Symbol)
	defer Log("Done handling GETBLOCK from %s for block %x on chain %s", args.From, args.Blockhash, args.Symbol)

	peerState := node.getPeerState(args.From)
	if peerState != ACTIVE && peerState != FOUND {
		Log("Received GETBLOCK from inactive or unknown peer %s for block %x on chain %s", args.From, args.Blockhash, args.Symbol)
		*reply = GetBlockReply{false, Block{}}
		go node.connectPeerIfNew(args.From)
	}

	block, err := node.bcs.GetBlock(args.Symbol, args.Blockhash)
	if err != nil {
		*reply = GetBlockReply{false, Block{}}
	} else {
		*reply = GetBlockReply{true, *block}
	}

	return nil
}

func (node *Node) SendGetBlock(peer *Peer, blockhash []byte, symbol string) (*Block, error) {
	args := &GetBlockArgs{blockhash, symbol, node.myIp}
	reply, err := node.callGetBlock(peer, args)
	node.handleRpcReply(peer, err)

	if err != nil {
		return nil, err
	} else if !reply.Success {
		return nil, errors.New("GETBLOCK unsuccessful")
	}

	return &reply.Block, nil
}

func (node *Node) callGetBlock(peer *Peer, args *GetBlockArgs) (*GetBlockReply, error) {
	Log("Sending GETBLOCK to %s for block %x on chain %s", peer.ip, args.Blockhash, args.Symbol)
	defer Log("Done sending GETBLOCK to %s for block %x on chain %s", peer.ip, args.Blockhash, args.Symbol)

	reply := &GetBlockReply{}
	err := peer.client.Call("Node.GetBlock", args, reply)
	return reply, err
}

//////////////////////////////////////////
// TX
// Share transaction with peers
// request: transaction data
// response: true if new

type TxArgs struct {
	Tx     GenericTransaction
	Symbol string
	From   string
}

func (node *Node) Tx(args *TxArgs, reply *bool) error {
	from := args.From
	if from == "" {
		from = "client"
	}

	Log("Received TX from %s %v", from, args.Tx)
	defer Log("Done handling TX from %s", from)

	new := node.bcs.AddTransactionToMempool(args.Tx, args.Symbol, true)

	if new {
		node.BroadcastTx(&args.Tx, args.Symbol)
		*reply = true
	} else {
		*reply = false
	}

	return nil
}

func (node *Node) GetBalance(args *GetBalanceRequest, reply *GetBalanceResponse) error {
	Log("Received GetBalance for address %v symbol %v", args.Address, args.Symbol)
	amount, ok := node.bcs.GetBalance(args.Symbol, args.Address)
	reply.Success = ok
	reply.Amount = amount
	return nil
}

func (node *Node) BroadcastTx(tx *GenericTransaction, symbol string) {
	args := &TxArgs{*tx, symbol, node.myIp}

	for _, peer := range node.getActivePeers() {
		go node.callTx(peer, args)
	}
}

func (node *Node) callTx(peer *Peer, args *TxArgs) {
	Log("Sending TX to %s %v", peer.ip, args.Tx)
	defer Log("Done sending TX to %s", peer.ip)

	var reply bool
	err := peer.client.Call("Node.Tx", args, &reply)
	node.handleRpcReply(peer, err)
}

////////////////////////////////
// Utils: Connecting

func (node *Node) connectPeerIfNew(peerIp string) (isNew bool, peer *Peer, err error) {
	node.mu.Lock()

	testPeer, ok := node.peers[peerIp]
	if peerIp == node.myIp || ok && (testPeer.state == ACTIVE || testPeer.state == FOUND) {
		node.mu.Unlock()
		return false, testPeer, nil
	}

	peer = &Peer{peerIp, FOUND, nil, time.Now()}
	node.peers[peerIp] = peer

	node.mu.Unlock()

	Log("Attempting to connect to peer %s", peerIp)

	client, err := rpc.Dial("tcp", peerIp)
	if err != nil {
		Log("Dialing error connecting to peer %s, (%s)", peerIp, err.Error())
		node.setPeerState(peerIp, INVALID)

		return true, nil, err
	}

	node.mu.Lock()
	peer.client = client
	node.mu.Unlock()

	node.SendVersion(peer)
	node.setPeerState(peerIp, ACTIVE)

	node.BroadcastAddr()

	ips := node.getPeerIps()
	SetSeeds(ips, node.myIp)
	Log("Connected peer %s, known peers: %v", peerIp, ips)

	return true, peer, nil
}

////////////////////////////////
// Utils: Chain Management

func (node *Node) reconcileChain(peerIp string, symbol string, theirBlockhashes [][]byte, theirHeight uint64) error {
	node.reconcileMu.Lock()
	defer node.reconcileMu.Unlock()

	node.bcs.chainsLock.RLock()

	bc, chainExists := node.bcs.chains[symbol]
	if !chainExists {
		Log("Attempted to reconcile non-existent chain %s with peer %s", symbol, peerIp)
		node.bcs.chainsLock.RUnlock()

		return errors.New("chain doesn't exist")
	}

	myHeight := bc.height
	bci := bc.Iterator()
	peer := node.peers[peerIp]

	height := myHeight
	theirIdx := theirHeight - myHeight
	block, _ := bci.Prev()

	Log("Reconciling chain %s with peer %s (myHeight %v, theirHeight %v)", symbol, peerIp, myHeight, theirHeight)
	defer Log("Finished reconciling chain %s with peer %s", symbol, peerIp)

	if theirHeight <= myHeight {
		Log("No reconciliation necessary for chain %s with peer %s", symbol, peerIp)
		node.bcs.chainsLock.RUnlock()

		return errors.New("no reconciliation necessary")
	}

	for int(theirIdx) < len(theirBlockhashes) && block != nil && !bytes.Equal(block.Hash, theirBlockhashes[theirIdx]) {
		height--
		theirIdx++
		block, _ = bci.Prev()
	}

	node.bcs.chainsLock.RUnlock()

	if int(theirIdx) > len(theirBlockhashes) {
		Log("Ran out of blockhashes reconciling chain %s with peer %s", symbol, peerIp)
		return errors.New("more blockhashes needed")
	}

	if block == nil {
		Log("Hit nil block reconciling chain %s with peer %s", symbol, peerIp)
		return errors.New("Hit nil block")
	}

	if height != myHeight {
		Log("Found fork at height %v while reconciling chain %s with peer %s", height, symbol, peerIp)
		node.bcs.RollbackToHeight(symbol, height, true)
	}

	for i := height + 1; i <= theirHeight; i++ {
		theirIdx--

		Log("Getting block at height %v on chain %s from peer %s", i, symbol, peerIp)
		block, err := node.SendGetBlock(peer, theirBlockhashes[theirIdx], symbol)

		if err != nil {
			Log(err.Error())
			return err
		}

		Log("Received block at height %v on chain %s from peer %s", i, symbol, peerIp)
		success := node.bcs.AddBlock(symbol, *block, true)

		if !success {
			Log("Conflict found at height %v on chain %s, stopping reconciliation with %s", i, symbol, peerIp)

			return errors.New("conflict found while reconciling")
		}
	}

	return nil
}

////////////////////////////////
// Utils: RPC Management

func (node *Node) handleRpcReply(peer *Peer, err error) {
	if err != nil {
		Log(err.Error())
		node.setPeerState(peer.ip, INVALID)
	}
}

////////////////////////////////
// Utils: State Access

func (node *Node) getActivePeers() []*Peer {
	node.mu.Lock()
	defer node.mu.Unlock()

	peers := make([]*Peer, 0)

	for _, peer := range node.peers {
		if peer.state == ACTIVE {
			peers = append(peers, peer)
		}
	}

	return peers
}

func (node *Node) getPeerIps() []string {
	node.mu.Lock()
	defer node.mu.Unlock()

	ips := make([]string, 0)

	for ip, peer := range node.peers {
		if peer.state == ACTIVE {
			ips = append(ips, ip)
		}
	}

	return ips
}

func (node *Node) getPeerState(peerIp string) PeerState {
	node.mu.Lock()
	defer node.mu.Unlock()

	peer, ok := node.peers[peerIp]
	if !ok {
		return UNKNOWN
	}

	return peer.state
}

func (node *Node) setPeerState(peerIp string, state PeerState) {
	node.mu.Lock()
	defer node.mu.Unlock()

	node.peers[peerIp].state = state
}
