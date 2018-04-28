package main

import "sync"

type ConsensusStateToken struct {
	openOrders []Order
	balances sync.Map
}

type ConsensusState struct {
	tokenStates sync.Map
	unconfirmedMatchIDs map[uint64]bool
	unconfirmedMatchIDsLock sync.RWMutex
	createdTokens []TokenInfo
	createdTokensLock sync.RWMutex
}

func (state *ConsensusState) AddOrder(symbol string, order Order) bool {
	return false
}

func (state *ConsensusState) RollbackOrder(symbol string, order Order) {

}

func (state *ConsensusState) AddCancelOrder(symbol string, cancelOrder CancelOrder) bool {
	return false
}

func (state *ConsensusState) RollbackCancelOrder(symbol string, cancelOrder CancelOrder) {

}

func (state *ConsensusState) AddTransactionConfirmed(symbol string, transactionConfirmed TransactionConfirmed) bool {
	return false
}

func (state *ConsensusState) RollbackTransactionConfirmed(symbol string, transactionConfirmed TransactionConfirmed) {

}

func (state *ConsensusState) AddTransfer(symbol string, transfer Transfer) bool {
	return false
}

func (state *ConsensusState) RollbackTransfer(symbol string, transfer Transfer) {

}

func (state *ConsensusState) AddMatch(match Match) bool {
	return false
}

func (state *ConsensusState) RollbackMatch(match Match) {

}

func (state *ConsensusState) AddCancelMatch(cancelMatch CancelMatch) bool {
	return false
}

func (state *ConsensusState) RollbackCancelMatch(cancelMatch CancelMatch) {

}

func (state *ConsensusState) AddCreateToken(createToken CreateToken) bool {
	return false
}

func (state *ConsensusState) RollbackCreateToken(createToken CreateToken) {

}
