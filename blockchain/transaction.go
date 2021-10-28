package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
)

type Transaction struct {
	ID      []byte
	Inputs  []TxnInput
	Outputs []TxnOutput
}

type TxnOutput struct {
	Value     int
	PublicKey string
}

type TxnInput struct {
	ID  []byte
	Out int
	Sig string
}

func (txn *Transaction) SetID() {
	var encoded bytes.Buffer
	var hash [32]byte
	encode := gob.NewEncoder(&encoded)
	err := encode.Encode(txn)
	Handle(err)

	hash = sha256.Sum256(encoded.Bytes())
	txn.ID = hash[:]
}

func (txn *Transaction) IsCoinbase() bool {
	return len(txn.Inputs) == 1 &&
		len(txn.Inputs[0].ID) == 0 &&
		txn.Inputs[0].Out == -1
}

func (in *TxnInput) CanUnlock(data string) bool {
	return in.Sig == data
}

func (out *TxnOutput) CanbeUnlocked(data string) bool {
	return out.PublicKey == data
}

func CoinbaseTxn(to string, data string) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Coiins to %s", to)
	}

	txnin := TxnInput{[]byte{}, -1, data}
	txnout := TxnOutput{100, to}

	txn := Transaction{nil, []TxnInput{txnin}, []TxnOutput{txnout}}
	txn.SetID()

	return &txn
}

func NewTransaction(from, to string, amount int, chain *BlockChain) *Transaction {
	var inputs []TxnInput
	var outputs []TxnOutput

	account, validOutputs := chain.FindSpendableOutputs(from, amount)

	if account < amount {
		log.Panic("Error: not enough funds")
	}

	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		Handle(err)

		for _, out := range outs {
			input := TxnInput{txID, out, from}
			inputs = append(inputs, input)
		}
	}

	outputs = append(outputs, TxnOutput{amount, to})

	if account > amount {
		outputs = append(outputs, TxnOutput{account - amount, from})
	}

	txn := Transaction{nil, inputs, outputs}
	txn.SetID()

	return &txn
}
