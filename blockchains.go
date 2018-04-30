package main

import (
	"errors"
	"sync"
	"bytes"
)

const MATCH_CHAIN = "MATCH"
const NATIVE_CHAIN = "NATIVE"

type Blockchains struct {
	chains sync.Map //map[string]*Blockchain
	consensusState ConsensusState
	locks map[string]*sync.Mutex
	dbFile string
}

type UncommittedTransactions struct {
	transactions []GenericTransaction
}

func (uncommitted *UncommittedTransactions) addTransaction(transaction GenericTransaction) {
	uncommitted.transactions = append(uncommitted.transactions, transaction)
}

func (uncommitted *UncommittedTransactions) undoTransactions(symbol string, state *ConsensusState) {
	for i := len(uncommitted.transactions) - 1; i >= 0; i-- {
		transaction := uncommitted.transactions[i].transaction
		switch uncommitted.transactions[i].transactionType {
		case ORDER: state.RollbackOrder(symbol, transaction.(Order))
		case CANCEL_ORDER: state.RollbackCancelOrder(transaction.(CancelOrder))
		case CLAIM_FUNDS: state.RollbackClaimFunds(symbol, transaction.(ClaimFunds))
		case TRANSFER: state.RollbackTransfer(symbol, transaction.(Transfer))
		case MATCH: state.RollbackMatch(transaction.(Match))
		case CREATE_TOKEN: state.RollbackCreateToken(transaction.(CreateToken))
		}
	}
}

func (blockchains *Blockchains) GetChain(symbol string) *Blockchain {
	result, _ := blockchains.chains.Load(symbol)
	return result.(*Blockchain)
}

func (blockchains *Blockchains) rollbackTokenToHeight(symbol string, height uint64) {
	if height <= blockchains.GetChain(symbol).GetStartHeight() {
		return
	}
	blocksToRemove := blockchains.GetChain(symbol).GetStartHeight() - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := blockchains.GetChain(symbol).RemoveLastBlock().(TokenData)
		for _, order := range removedData.Orders {
			blockchains.consensusState.RollbackOrder(symbol, order)
		}

		for _, claimFunds := range removedData.ClaimFunds {
			blockchains.consensusState.RollbackClaimFunds(symbol, claimFunds)
		}

		for _, transfer := range removedData.Transfers {
			blockchains.consensusState.RollbackTransfer(symbol, transfer)
		}
	}
}

func (blockchains *Blockchains) rollbackMatchToHeight(height uint64) {
	if height <= blockchains.GetChain(MATCH_CHAIN).GetStartHeight() {
		return
	}

	blocksToRemove := blockchains.GetChain(MATCH_CHAIN).GetStartHeight() - height
	for i := uint64(0); i < blocksToRemove; i++ {
		removedData := blockchains.GetChain(MATCH_CHAIN).RemoveLastBlock().(MatchData)
		for _, match := range removedData.Matches {
			blockchains.consensusState.RollbackMatch(match)
		}

		for _, cancelOrder := range removedData.CancelOrders {
			blockchains.consensusState.RollbackCancelOrder(cancelOrder)
		}

		for _, createToken := range removedData.CreateTokens {
			blockchains.consensusState.RollbackCreateToken(createToken)
		}
	}
}

func (blockchains *Blockchains) RollbackToHeight(symbol string, height uint64) {
	blockchains.locks[symbol].Lock()
	defer blockchains.locks[symbol].Unlock()
	if symbol == MATCH_CHAIN {
		blockchains.rollbackMatchToHeight(height)
	} else {
		blockchains.rollbackTokenToHeight(symbol, height)
	}
}

func (blockchains *Blockchains) addTokenData(symbol string, tokenData TokenData, uncommitted *UncommittedTransactions) bool {
	for _, order := range tokenData.Orders {
		if !blockchains.consensusState.AddOrder(symbol, order) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     order,
			transactionType: ORDER,
		})
	}

	for _, claimFunds := range tokenData.ClaimFunds {
		if !blockchains.consensusState.AddClaimFunds(symbol, claimFunds) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     claimFunds,
			transactionType: CLAIM_FUNDS,
		})
	}

	for _, transfer := range tokenData.Transfers {
		if !blockchains.consensusState.AddTransfer(symbol, transfer) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     transfer,
			transactionType: TRANSFER,
		})
	}
	return true
}

func (blockchains *Blockchains) addMatchData(matchData MatchData, uncommitted *UncommittedTransactions) bool {
	for _, match := range matchData.Matches {
		if !blockchains.consensusState.AddMatch(match) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     match,
			transactionType: MATCH,
		})
	}

	for _, cancelOrder := range matchData.CancelOrders {
		if !blockchains.consensusState.AddCancelOrder(cancelOrder) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     cancelOrder,
			transactionType: CANCEL_ORDER,
		})
	}

	for _, createToken := range matchData.CreateTokens {
		if !blockchains.consensusState.AddCreateToken(createToken) {
			return false
		}
		uncommitted.addTransaction(GenericTransaction{
			transaction:     createToken,
			transactionType: CREATE_TOKEN,
		})
	}
	return true
}

func (blockchains *Blockchains) AddBlock(symbol string, block Block) {
	blockchains.AddBlocks(symbol, []Block{block})
}

func (blockchains *Blockchains) AddBlocks(symbol string, blocks []Block) bool {
	blockchains.locks[symbol].Lock()
	defer blockchains.locks[symbol].Unlock()
	blocksAdded := 0
	var uncommitted UncommittedTransactions
	failed := false
	for _, block := range blocks {
		if !bytes.Equal(blockchains.GetChain(symbol).tipHash, block.PrevBlockHash) {
			failed = true
			break
		}
		if symbol == MATCH_CHAIN {
			if !blockchains.addMatchData(block.Data.(MatchData), &uncommitted) {
				failed = true
				break
			}
		} else {
			if !blockchains.addTokenData(symbol, block.Data.(TokenData), &uncommitted) {
				failed = true
				break
			}
		}
		blockchains.GetChain(symbol).AddBlock(block)
		blocksAdded++
	}
	if failed {
		uncommitted.undoTransactions(symbol, &blockchains.consensusState)
		for i := 0; i < blocksAdded; i++ {
			blockchains.GetChain(symbol).RemoveLastBlock()
		}
		return false
	}
	return true
}

func (blockchains *Blockchains) GetOpenOrders(symbol string) sync.Map {
	blockchains.locks[symbol].Lock()
	defer blockchains.locks[symbol].Unlock()
	state, _ := blockchains.consensusState.tokenStates.Load(symbol)
	return state.(ConsensusStateToken).openOrders
}

func (blockchains *Blockchains) GetBalance(symbol string, address string) (uint64, bool) {
	blockchains.locks[symbol].Lock()
	defer blockchains.locks[symbol].Unlock()
	state, _ := blockchains.consensusState.tokenStates.Load(symbol)
	balance, ok := state.(*ConsensusStateToken).balances[address]
	return balance, ok
}

func (blockchains *Blockchains) AddTokenChain(createToken CreateToken) {
	chain := NewTokenChain(blockchains.dbFile, createToken.TokenInfo.Symbol)
	blockchains.chains.Store(createToken.TokenInfo.Symbol, chain)
	blockchains.locks[createToken.TokenInfo.Symbol] = &sync.Mutex{}
	blockchains.AddBlock(createToken.TokenInfo.Symbol, *NewTokenGenesisBlock(createToken))
}

func CreateNewBlockchains(dbName string) *Blockchains {
	//instantiates state and blockchains
	blockchains := &Blockchains{}
	blockchains.dbFile = dbName
	blockchains.chains.Store(MATCH_CHAIN, NewBlockchain(dbName))
	blockchains.locks[MATCH_CHAIN] = &sync.Mutex{}
	blockchains.AddBlock(MATCH_CHAIN, *NewGenesisBlock())
	return blockchains
}

func (blockchains *Blockchains) GetHeight(symbol string) uint64 {
	blockchains.locks[symbol].Lock()
	defer blockchains.locks[symbol].Unlock()
	return blockchains.GetChain(symbol).GetStartHeight()
}

func (blockchains *Blockchains) GetBlock(symbol string, blockhash []byte) (*Block, error) {
	bc, ok := blockchains.chains.Load(symbol)
	if !ok {
		return nil, errors.New("invalid chain")
	}

	block, blockErr := bc.(*Blockchain).GetBlock(blockhash)
	return block, blockErr
}

func (blockchains *Blockchains) GetHeights() map[string]uint64 {
	heights := make(map[string]uint64)
	blockchains.chains.Range(func(symbol, chain interface{}) bool {
		heights[symbol.(string)] = chain.(*Blockchain).GetStartHeight()
		return true
	})
	return heights
}

func (blockchains *Blockchains) GetBlockhashes() map[string][][]byte {
	blockhashes := make(map[string][][]byte)

	blockchains.chains.Range(func(symbol, chain interface{}) bool {
		blockhashes[symbol.(string)] = chain.(*Blockchain).GetBlockhashes()
		return true
	})
	return blockhashes
}
