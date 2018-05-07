package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
)

func Respond(w http.ResponseWriter, code int, payload interface{}) {
	ret := make(map[string]interface{})
	if code >= 200 && code < 300 {
		ret["result"] = "success"
	} else {
		ret["result"] = "failure"
	}

	if payload != nil {
		ret["response"] = payload
	}

	response, _ := json.Marshal(ret)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func RespondWithError(w http.ResponseWriter, code int, msg string) {
	Respond(w, code, map[string]string{"error": msg})
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("404: %s %s", r.Method, r.URL)
	RespondWithError(w, 404, "Not found")
}

func CreateAddressHandler(config *Config, db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		with_private := r.URL.Query().Get("with_private")

		pub, priv, err := CreateAddress()
		if err != nil {
			RespondWithError(w, 500, fmt.Sprintf("Could not create a new key: %v", err))
			return
		}

		err = db.InsertKey(pub, priv)
		if err != nil {
			RespondWithError(w, 500, fmt.Sprintf("Could not save newly created key: %v", err))
			return
		}

		log.Printf("Created address: %v", pub)

		if with_private == "true" {
			Respond(w, 200, map[string]string{"address": pub, "private": priv})
		} else {
			Respond(w, 200, map[string]string{"address": pub})
		}

	}
}

func GetBalanceHandler(config *Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var balance *big.Int
		var err error

		address := r.URL.Query().Get("address")
		contractAddress := r.URL.Query().Get("contract")

		if address == "" {
			RespondWithError(w, 400, "Missing 'address' field")
			return
		}

		if contractAddress == "" {
			// Retrieve ETH balance
			balanceFloat, err := GetAddressBalance(config, address)
			if err != nil {
				RespondWithError(w, 500, fmt.Sprintf("Could not retrieve ethereum balance: %v", err))
				return
			}

			Respond(w, 200, map[string]string{"balance": balanceFloat.Text('f', 10)})
		} else {
			// Retrieve erc20 balance for given address
			balance, err = GetERC20AddressBalance(config, address, contractAddress)
			if err != nil {
				RespondWithError(w, 500, fmt.Sprintf("Could not retrieve ethereum balance: %v", err))
				return
			}

			Respond(w, 200, map[string]string{"balance": balance.Text(10)})
		}
	}
}

func SendEthHandler(config *Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			log.Printf("SendEthHandler: Could not parse body parameters")
			RespondWithError(w, 400, "Could not parse parameters")
			return
		}

		address := r.Form.Get("address")
		private := r.Form.Get("private")
		amount := r.Form.Get("amount")

		if address == "" {
			log.Printf("Got Send Ethereum order but 'address' field is missing")
			RespondWithError(w, 400, "Missing 'address' field")
			return
		}

		if private == "" {
			log.Printf("Got Send Ethereum order but 'private' field is missing")
			RespondWithError(w, 400, "Missing 'private' field")
			return
		}

		if amount == "" {
			log.Printf("Got Send Ethereum order but 'amount' field is missing")
			RespondWithError(w, 400, "Missing 'amount' field")
			return
		}

		f, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			RespondWithError(w, 400, "Could not convert amount")
			return
		}

		bgAmount := new(big.Float)
		bgAmount.SetFloat64(f)

		bgEthWei := new(big.Float)
		bgEthWei.SetString("1000000000000000000")

		bgAmount = bgAmount.Mul(bgAmount, bgEthWei)
		bgAmountInt, _ := bgAmount.Int(new(big.Int))

		tx, err := SendEthCoin(config, bgAmountInt, private, address)
		if err != nil {
			RespondWithError(w, 500, fmt.Sprintf("Could not send Ethereum coin: %v", err))
			return
		}

		Respond(w, 200, map[string]string{"txhash": tx})
	}
}

func SendERC20Handler(config *Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			log.Printf("SendERC20Handler: Could not parse body parameters")
			RespondWithError(w, 400, "Could not parse parameters")
			return
		}

		address := r.Form.Get("address")
		contract := r.Form.Get("contract")
		private := r.Form.Get("private")
		amount := r.Form.Get("amount")

		if address == "" {
			log.Printf("Got Send Ethereum order but 'address' field is missing")
			RespondWithError(w, 400, "Missing 'address' field")
			return
		}

		if contract == "" {
			log.Printf("Got Send Ethereum order but 'contract' field is missing")
			RespondWithError(w, 400, "Missing 'contract' field")
			return
		}

		if private == "" {
			log.Printf("Got Send Ethereum order but 'private' field is missing")
			RespondWithError(w, 400, "Missing 'private' field")
			return
		}

		if amount == "" {
			log.Printf("Got Send Ethereum order but 'amount' field is missing")
			RespondWithError(w, 400, "Missing 'amount' field")
			return
		}

		bgAmount := new(big.Int)
		bgAmount.UnmarshalText([]byte(amount))

		tx, err := SendERC20Token(config, bgAmount, contract, private, address)
		if err != nil {
			RespondWithError(w, 500, fmt.Sprintf("Could not send ERC20 token: %v", err))
			return
		}

		Respond(w, 200, map[string]string{"txhash": tx})
	}
}

func GetNotificationsHandler(config *Config, db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		remove := r.URL.Query().Get("remove")

		// Retrieve next 100 records in database
		notifications, err := db.GetNotifications(remove == "true")
		if err != nil {
			log.Printf("GetNotificationsHandler: %v", err)
			RespondWithError(w, 500, "Could retrieve notifications")
			return
		}

		Respond(w, 200, notifications)
	}
}
