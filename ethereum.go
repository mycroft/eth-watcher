package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/ethereum/go-ethereum/crypto/sha3"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(btcec.S256(), rand.Reader)
}

func Keccak256(in []byte) []byte {
	hash := sha3.NewKeccak256()

	hash.Write(in)

	return hash.Sum(nil)
}

func Prepend(in []byte, size int) []byte {
	if len(in) == size {
		return in
	}

	prefix := make([]byte, size-len(in))

	return append(prefix, in...)
}

func ImportPrivateHex(h string) (*ecdsa.PrivateKey, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}

	key, err := GenerateKey()
	if err != nil {
		return nil, err
	}

	key.D.SetBytes(b)

	return key, nil
}

func PrivateKeyToAddress(key *ecdsa.PrivateKey) (string, string) {
	var publickey bytes.Buffer

	publickey.Write(Prepend(key.PublicKey.X.Bytes(), 32))
	publickey.Write(Prepend(key.PublicKey.Y.Bytes(), 32))

	privatekey := Prepend(key.D.Bytes(), 32)

	hash := Keccak256(publickey.Bytes())

	return fmt.Sprintf("%x", hash[12:]), fmt.Sprintf("%x", privatekey)
}

func CreateAddress() (string, string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", "", err
	}

	address, private := PrivateKeyToAddress(key)

	return address, private, nil
}

func PrivateHexToAddress(private string) (string, error) {
	key, err := crypto.HexToECDSA(private)
	if err != nil {
		return "", err
	}

	address, privateVerify := PrivateKeyToAddress(key)

	if private != privateVerify {
		return "", fmt.Errorf("Invalid private verification")
	}

	return address, nil
}

func IsAddress(address string) bool {
	return common.IsHexAddress(address)
}

func ConnectRPC(config *Config) (*ethclient.Client, error) {
	client, err := ethclient.Dial(fmt.Sprintf("http://%s", config.RPCURL))
	if err != nil {
		return nil, fmt.Errorf("Could not connect to Ethereum RPC API: %v", err)
	}

	return client, nil
}

func GetAddressBalance(config *Config, address string) (*big.Float, error) {
	var context = context.Background()

	client, err := ConnectRPC(config)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	bgInt, err := client.BalanceAt(context, common.HexToAddress(address), nil)

	bgWei := new(big.Float)
	bgWei.UnmarshalText([]byte("0.000000000000000001"))

	bgFloat := new(big.Float)
	bgFloat.SetInt(bgInt)
	bgFloat = bgFloat.Mul(bgWei, bgFloat)

	return bgFloat, nil
}

func GetERC20AddressBalance(config *Config, address string, contractAddress string) (*big.Int, error) {
	client, err := ConnectRPC(config)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	token, err := NewToken(common.HexToAddress(contractAddress), client)
	if err != nil {
		return nil, fmt.Errorf("Failed to instantiate a Token contract: %v", err)
	}

	balance, err := token.BalanceOf(nil, common.HexToAddress(address))
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve balance for token: %v", err)
	}

	return balance, nil
}

func SendEthCoin(config *Config, amount *big.Int, private string, address string) (string, error) {
	ctx := context.Background()
	signer := types.HomesteadSigner{}

	client, err := ConnectRPC(config)
	if err != nil {
		return "", err
	}
	defer client.Close()

	key, err := crypto.HexToECDSA(private)
	if err != nil {
		return "", err
	}

	nonce, err := client.NonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey), nil)
	if err != nil {
		return "", err
	}

	tx := types.NewTransaction(
		nonce,
		common.HexToAddress(address),
		amount,       // amount
		60000,        // gasLimit
		new(big.Int), // gasPrice
		[]byte(""),
	)

	signature, err := crypto.Sign(signer.Hash(tx).Bytes(), key)
	if err != nil {
		return "", fmt.Errorf("Signature creation error: %v", err)
	}

	signedTx, err := tx.WithSignature(signer, signature)
	if err != nil {
		return "", fmt.Errorf("Signer with signature error: %v", err)
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", fmt.Errorf("Send tx error: %v", err)
	}

	return signedTx.Hash().String(), nil
}

func SendERC20Token(config *Config, amount *big.Int, contractAddress, private, address string) (string, error) {
	client, err := ConnectRPC(config)
	if err != nil {
		return "", err
	}
	defer client.Close()

	token, err := NewToken(common.HexToAddress(contractAddress), client)
	if err != nil {
		return "", fmt.Errorf("Failed to instantiate a Token contract: %v", err)
	}

	key, err := crypto.HexToECDSA(private)
	if err != nil {
		return "", err
	}

	auth := bind.NewKeyedTransactor(key)

	tx, err := token.Transfer(auth, common.HexToAddress(address), amount)
	if err != nil {
		return "", err
	}

	return tx.Hash().String(), nil
}

func GetTransactionFrom(tx *types.Transaction) (common.Address, error) {
	var signer types.Signer
	signer = types.HomesteadSigner{}

	v, _, _ := tx.RawSignatureValues()

	if v.Sign() != 0 && tx.Protected() {
		signer = types.NewEIP155Signer(tx.ChainId())
	}

	msg, err := tx.AsMessage(signer)
	if err != nil {
		return common.Address{}, err
	}

	return msg.From(), nil
}

func GetContractDestAddress(data []byte) (common.Address, *big.Int, error) {
	var destAddr common.Address

	if len(data) < 4 {
		return common.Address{}, nil, fmt.Errorf("Data size too short")
	}

	transferAddr := []byte{0xa9, 0x05, 0x9c, 0xbb}
	transferFromAddr := []byte{0x23, 0xb8, 0x72, 0xdd}

	if false == bytes.Equal(transferAddr, data[:4]) && false == bytes.Equal(transferFromAddr, data[:4]) {
		return common.Address{}, nil, fmt.Errorf("Could not find a valid operation")
	}

	command := data[:4]
	data = data[4:]
	params := make([][]byte, 0)
	value := new(big.Int)

	for len(data) > 0 {
		if len(data) < 32 {
			params = append(params, make([]byte, 32-len(data)))
			params = append(params, data[:len(data)])
			data = data[len(data):]
		} else {
			params = append(params, data[:32])
			data = data[32:]
		}
	}

	if bytes.Equal(transferAddr, command) {
		if len(params) != 2 {
			return common.Address{}, nil, fmt.Errorf("Invalid contract transaction parameters")
		}

		destAddr = common.BytesToAddress(params[0])
		value.SetBytes(params[1])
	} else if bytes.Equal(transferFromAddr, command) {
		if len(params) != 3 {
			return common.Address{}, nil, fmt.Errorf("Invalid contract transaction parameters")
		}

		destAddr = common.BytesToAddress(params[1])
		value.SetBytes(params[2])
	}

	return destAddr, value, nil
}

func ParseTransaction(tx *types.Transaction, isPending bool) (NotifyMessage, error) {
	var value *big.Int
	var contractDest common.Address

	if tx == nil {
		return NotifyMessage{}, fmt.Errorf("Transaction is nil: Can't parse.")
	}

	if tx.To() == nil {
		log.Printf("Transaction(%s).To() is nil: Can't parse (Happens on contract creation).", tx.Hash().Hex())

		return NotifyMessage{}, nil
	}

	from, err := GetTransactionFrom(tx)
	if err != nil {
		return NotifyMessage{}, err
	}

	dest, value, err := GetContractDestAddress(tx.Data())
	if err != nil {
		dest = *tx.To()
		value = tx.Value()

		if fDebug {
			log.Printf("ETH: %x => %x / Value: %s (pending:%v)\n", from.String(), dest.String(), value.Text(10), isPending)
		}

		return NotifyMessage{
			MessageType:     NOTIFY_TYPE_TX,
			AddressFrom:     from.Hex()[2:],
			AddressTo:       dest.Hex()[2:],
			Amount:          value,
			ContractAddress: "",
			IsPending:       isPending,
			TxHash:          tx.Hash().Hex()[2:],
		}, nil
	} else {
		contractDest = *tx.To()

		if fDebug {
			log.Printf("Contract(%x): %x => %x / Value: %s (pending:%v)\n", contractDest.String(), from.String(), dest.String(), value.Text(10), isPending)
		}

		return NotifyMessage{
			MessageType:     NOTIFY_TYPE_TX,
			AddressFrom:     from.Hex()[2:],
			AddressTo:       dest.Hex()[2:],
			Amount:          value,
			ContractAddress: contractDest.Hex()[2:],
			IsPending:       isPending,
			TxHash:          tx.Hash().Hex()[2:],
		}, nil
	}

	return NotifyMessage{}, nil
}

func ReadTransaction(client *ethclient.Client, hashStr string) (NotifyMessage, error) {
	hash := common.HexToHash(hashStr)

	tx, pending, err := client.TransactionByHash(context.Background(), hash)
	if err != nil {
		return NotifyMessage{}, fmt.Errorf("ReadTransaction(%s) failed: %v", hash.Hex(), err)
	}

	return ParseTransaction(tx, pending)
}

func ReadBlock(client *ethclient.Client, hashStr string, number *big.Int) (*big.Int, []NotifyMessage, error) {
	var block *types.Block
	var err error
	messages := make([]NotifyMessage, 0)

	if hashStr != "" {
		hash := common.HexToHash(hashStr)

		block, err = client.BlockByHash(context.Background(), hash)
		if err != nil {
			return nil, messages, fmt.Errorf("ReadBlock failed: %v", err)
		}
	} else {
		block, err = client.BlockByNumber(context.Background(), number)
		if err != nil {
			return nil, messages, fmt.Errorf("ReadBlock failed: %v", err)
		}
	}

	for _, tx := range block.Transactions() {
		message, err := ParseTransaction(tx, false)
		if err != nil {
			return block.Number(), messages, err
		}

		messages = append(messages, message)
	}

	return block.Number(), messages, nil
}
