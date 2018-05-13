package main

import (
	"flag"
	"log"
	"math/big"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

const (
	NOTIFY_TYPE_NONE = iota
	NOTIFY_TYPE_TX
	NOTIFY_TYPE_ADMIN
)

type NotifyMessage struct {
	MessageType     int
	AddressFrom     string
	AddressTo       string
	Amount          *big.Int
	ContractAddress string
	IsPending       bool
	TxHash          string
}

var (
	fDebug      bool
	fInit       bool
	fConfigFile string
)

func init() {
	flag.BoolVar(&fInit, "init", false, "DB Init")
	flag.BoolVar(&fDebug, "debug", false, "Debug")
	flag.StringVar(&fConfigFile, "config", "config.ini", "Configuration file")
}

func main() {
	var last_id uint64

	flag.Parse()

	config, err := LoadConfiguration(fConfigFile)
	if err != nil {
		panic(err)
	}

	db, err := DbOpen(config)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if fInit {
		err = db.InitTables()
		if err != nil {
			panic(err)
		}
		log.Println("Schema created in database.")

		return
	}

	last_id_str, err := db.GetSetting("last_block")
	if err != nil {
		log.Println("Warning: Could not get last block id parsed from database: No recovery.")
		last_id = 0
	} else {
		last_id, err = strconv.ParseUint(last_id_str, 10, 64)
		if err != nil {
			log.Printf("Warning: Could not convert %s as integer", last_id_str)
			last_id = 0
		}
	}

	r := mux.NewRouter()
	r.HandleFunc("/createAddress", CreateAddressHandler(config, db)).Methods("POST")
	r.HandleFunc("/registerAddress", RegisterAddressHandler(config, db)).Methods("POST")
	r.HandleFunc("/getBalance", GetBalanceHandler(config))
	r.HandleFunc("/sendEth", SendEthHandler(config))
	r.HandleFunc("/sendErc20", SendERC20Handler(config, db))
	r.HandleFunc("/getNotifications", GetNotificationsHandler(config, db))

	r.NotFoundHandler = http.HandlerFunc(NotFoundHandler)

	ch := make(chan NotifyMessage, 1024)

	go Notifier(config, db, ch)
	go Subscriber(config, ch, last_id)

	log.Println("Starting webserver...")
	http.ListenAndServe(":8080", r)
}
