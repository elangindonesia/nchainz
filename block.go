package main

import (
	"bytes"
	"encoding/gob"
	"time"
)

type TokenType uint8

const (
	BNB TokenType = iota + 1
	BTC
)

type BlockType uint8

const (
	MATCH BlockType = iota + 1
	ORDER
	TRANSFER
	CANCEL
	STRING
)

type Block struct {
	Timestamp     int64
	Type          BlockType
	Data          BlockData
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
}

type BlockData interface{}

type MatchData struct {
	Matches []Match
	Cancels []Cancel
}

type TokenData struct {
	Orders    []Order
	Cancels   []Cancel
	Transfers []Transfer
}

type Match struct {
	SellTokenType     TokenType
	BuyTokenType      TokenType
	AmountSold        uint64
	SellerBlockIndex  uint64
	BuyerBlockIndex   uint64
	SellerOrderOffset uint8
	BuyerOrderOffset  uint8
	SellerBlockHash   []byte
	BuyerBlockHash    []byte
}

type Order struct {
	BuyTokenType  TokenType
	AmountToSell  uint64
	AmountToBuy   uint64
	SellerAddress []byte
	Signature     []byte
}

type Transfer struct {
	Amount      uint64
	FromAddress []byte
	ToAddress   []byte
	Signature   []byte
}

type Cancel struct {
	BlockIndex  uint64
	BlockOffset uint8
	Signature   []byte
}

func GetBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func NewBlock(data string, prevBlockHash []byte) *Block {
	block := &Block{time.Now().Unix(), STRING, []byte(data), prevBlockHash, []byte{}, 0}

	// Add block
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()
	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}