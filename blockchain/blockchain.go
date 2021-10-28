package blockchain

import (
	"encoding/hex"
	"fmt"
	"os"
	"runtime"

	badger "github.com/dgraph-io/badger/v3"
)

const (
	dbPath      = "./tmp/blocks"
	dbFile      = "./tmp/blocks/MANIFEST"
	genesisData = "First Transaction form Genesis"
)

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

func DBexists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}
	return true
}

func InitBlockChain(address string) *BlockChain {

	if DBexists() {
		fmt.Println("Blochchain already exists")
		runtime.Goexit()
	}
	var lastHash []byte

	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil

	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		cbtxn := CoinbaseTxn(address, genesisData)
		genesis := Genesis(cbtxn)
		fmt.Println("Genesis created")
		err = txn.Set(genesis.Hash, genesis.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), genesis.Hash)

		lastHash = genesis.Hash

		return err
	})

	Handle(err)

	blockchain := BlockChain{lastHash, db}
	return &blockchain
}

func ContinueBlockChain(address string) *BlockChain {
	if !DBexists() {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}
	var lastHash []byte

	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil

	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)

		err = item.Value(func(val []byte) error {
			lastHash = append([]byte{}, val...)
			return nil
		})
		return err
	})
	Handle(err)

	return &BlockChain{lastHash, db}
}

func (chain *BlockChain) AddBlock(transactions []*Transaction) {
	var lastHash []byte

	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = append([]byte{}, val...)
			return nil
		})
		return err
	})
	Handle(err)

	newBlock := CreateBlock(transactions, lastHash)

	err = chain.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), newBlock.Hash)

		chain.LastHash = newBlock.Hash

		return err
	})
	Handle(err)
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	iter := &BlockChainIterator{chain.LastHash, chain.Database}

	return iter
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block

	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		Handle(err)
		var encodedBlock []byte
		err = item.Value(func(val []byte) error {
			encodedBlock = append([]byte{}, val...)
			return nil
		})
		block = Deserialize(encodedBlock)

		return err
	})
	Handle(err)

	iter.CurrentHash = block.PrevHash

	return block
}

func (chain *BlockChain) FindUnspentTransactions(address string) []Transaction {
	var unspentTxns []Transaction
	spentTxnOs := make(map[string][]int)

	iter := chain.Iterator()

	for {
		block := iter.Next()

		for _, txn := range block.Transactions {
			txnID := hex.EncodeToString(txn.ID)

		Outputs:
			for outIdx, out := range txn.Outputs {
				if spentTxnOs[txnID] != nil {
					for _, spentOut := range spentTxnOs[txnID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}
				if out.CanbeUnlocked(address) {
					unspentTxns = append(unspentTxns, *txn)
				}
			}
			if !txn.IsCoinbase() {
				for _, in := range txn.Inputs {
					if in.CanUnlock(address) {
						inTxnID := hex.EncodeToString(in.ID)
						spentTxnOs[inTxnID] = append(spentTxnOs[inTxnID], in.Out)
					}
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}
	return unspentTxns
}

func (chain *BlockChain) FindUTxnO(address string) []TxnOutput {
	var UTxnOs []TxnOutput
	unspentTransactions := chain.FindUnspentTransactions(address)

	for _, txn := range unspentTransactions {
		for _, out := range txn.Outputs {
			if out.CanbeUnlocked(address) {
				UTxnOs = append(UTxnOs, out)
			}
		}
	}
	return UTxnOs
}

func (chain *BlockChain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	unspendTxns := chain.FindUnspentTransactions(address)
	accumulated := 0

Work:
	for _, txn := range unspendTxns {
		txnID := hex.EncodeToString(txn.ID)

		for outIdx, out := range txn.Outputs {
			if out.CanbeUnlocked(address) && accumulated < amount {
				accumulated += out.Value
				unspentOuts[txnID] = append(unspentOuts[txnID], outIdx)

				if accumulated >= amount {
					break Work
				}
			}
		}
	}

	return accumulated, unspentOuts
}
