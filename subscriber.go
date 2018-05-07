package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gorilla/websocket"
)

type SubscriptionMessage struct {
	Id     int         `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type SubscriptionResponse struct {
	Id     int    `json:"id"`
	Result string `json:"result"`
}

type BlockHeader struct {
	ParentHash string `json:"parentHash"`
	Difficulty string `json:"difficulty"`
	Number     string `json:"number"`
	GasLimit   string `json:"gasLimit"`
	GasUsed    string `json:"gasUsed"`
	Timestamp  string `json:"timestamp"`
	Hash       string `json:"hash"`
}

type Params struct {
	Subscription string      `json:"subscription"`
	Result       interface{} `json:"result`
}

type ResponseMessage struct {
	JsonRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  Params `json:"params"`
}

const (
	TYPE_BLOCK_HASH = iota
	TYPE_TXN_HASH
)

type ObjMessage struct {
	Type   int
	Hash   string
	Number *big.Int
}

func GetSubscriptionMessage(messageId int, subscription string) ([]byte, error) {
	params := make([]interface{}, 0)
	params = append(params, subscription)

	m := SubscriptionMessage{
		messageId,
		"eth_subscribe",
		params,
	}

	b, err := json.Marshal(m)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func SendMessage(c *websocket.Conn, messageId int, subscription string) (string, error) {
	var resp SubscriptionResponse

	w, err := c.NextWriter(websocket.TextMessage)
	if err != nil {
		return "", err
	}

	message, err := GetSubscriptionMessage(messageId, subscription)
	if err != nil {
		return "", err
	}

	w.Write(message)
	w.Write([]byte{'\n'})
	w.Close()

	_, message, err = c.ReadMessage()
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(message, &resp)
	if err != nil {
		return "", err
	}

	return resp.Result, err
}

func ConnectWS(config *Config, ch chan<- ObjMessage) error {
	var MessageId int
	MessageId = 1

	log.Printf("Connecting to Ethereum Websocket")

	u := url.URL{Scheme: "ws", Host: config.WebsocketURL, Path: "/"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("ConnectWS dial: %v", err)
	}
	defer c.Close()

	subHashHeads, err := SendMessage(c, MessageId, "newHeads")
	if err != nil {
		return fmt.Errorf("SendMessage: newHeads: %v", err)
	}

	MessageId += 1

	subHashTransactions, err := SendMessage(c, MessageId, "newPendingTransactions")
	if err != nil {
		return fmt.Errorf("SendMessage: newPendingTransactions: %v", err)
	}

	log.Printf("ConnectWS: Connected. Subscriptions are %s and %s", subHashHeads[:12], subHashTransactions[:12])

	for {
		var response ResponseMessage

		_, message, err := c.ReadMessage()
		if err != nil {
			return err
		}

		err = json.Unmarshal(message, &response)
		if err != nil {
			return fmt.Errorf("Could not decode message/parse json: %v", err)
		}

		if response.Params.Subscription == subHashTransactions {
			txHash := response.Params.Result.(string)

			ch <- ObjMessage{TYPE_TXN_HASH, txHash, nil}
		} else {
			var Header BlockHeader
			response.Params.Result = &Header

			err = json.Unmarshal(message, &response)
			if err != nil {
				return fmt.Errorf("Could not decode message/parse json: %v", err)
			}

			bgInt, err := hexutil.DecodeBig(Header.Number)
			if err != nil {
				return fmt.Errorf("Could not decode block number: %v", err)
			}

			ch <- ObjMessage{TYPE_BLOCK_HASH, Header.Hash, bgInt}
		}
	}
}

func Listener(config *Config, ch <-chan ObjMessage, notifyChannel chan<- NotifyMessage, last_id uint64) {
	client, err := ConnectRPC(config)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	for message := range ch {
		switch message.Type {
		case TYPE_BLOCK_HASH:
			// Recovery: We get a recent block, but the last we parsed is more older that current - 1
			if last_id != 0 {
				last := new(big.Int)
				last.SetUint64(last_id + 1)

				for 0 != last.Cmp(message.Number) {
					log.Printf("Recovery: Doing block %s", last.Text(10))
					_, txns, err := ReadBlock(client, "", last)
					if err != nil {
						log.Println("Listener:", err)
						continue
					}

					for _, txn := range txns {
						notifyChannel <- txn
					}

					notifyChannel <- NotifyMessage{
						MessageType: NOTIFY_TYPE_ADMIN,
						Amount:      last,
					}

					last.SetUint64(last.Uint64() + 1)
				}

				// We set last_id a 0. We don't want this process to restart.
				log.Printf("Recovery is over: Done up to block %s", last.Text(10))

				last_id = 0
			}

			// Retrieve the block, and check all transactions
			last, txns, err := ReadBlock(client, message.Hash, nil)
			if err != nil {
				log.Println("Listener:", err)
				continue
			}

			for _, txn := range txns {
				notifyChannel <- txn
			}

			notifyChannel <- NotifyMessage{
				MessageType: NOTIFY_TYPE_ADMIN,
				Amount:      last,
			}

		case TYPE_TXN_HASH:
			txn, err := ReadTransaction(client, message.Hash)
			if err != nil {
				log.Println("Listener:", err)
				continue
			}

			notifyChannel <- txn
		}
	}
}

func Notifier(config *Config, db *DB, ch <-chan NotifyMessage) {
	for message := range ch {
		if message.MessageType == NOTIFY_TYPE_NONE {
			continue
		}

		if message.MessageType == NOTIFY_TYPE_ADMIN {
			db.SetSetting("last_block", message.Amount.Text(10))
			continue
		}

		isKnown, err := db.IsAddressKnown(message.AddressTo)
		if err != nil {
			log.Println(err)
			continue
		}

		if false == isKnown {
			continue
		}

		err = db.InsertNotification(
			message.AddressFrom,
			message.AddressTo,
			message.ContractAddress,
			message.Amount.Text(10),
			message.IsPending,
			message.TxHash,
		)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

func Subscriber(config *Config, notifyChannel chan<- NotifyMessage, last_id uint64) {
	ch := make(chan ObjMessage, 1024)

	go Listener(config, ch, notifyChannel, last_id)

	for {
		ts_startup := time.Now()
		err := ConnectWS(config, ch)
		if err != nil {
			log.Println(err)
		}

		elapsed := time.Now().Sub(ts_startup)

		if elapsed < time.Second*5 {
			// Wait a few seconds before retrying
			time.Sleep(time.Second * 5)
		}
	}
}
