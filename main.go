package main

import (
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/common"
	"github.com/binance-chain/tss-lib/ecdsa/keygen"
	"github.com/binance-chain/tss-lib/ecdsa/signing"
	"github.com/binance-chain/tss-lib/test"
	"github.com/binance-chain/tss-lib/tss"
	"io/fs"
	"log"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var (
	curve     elliptic.Curve
	count     int
	threshold int

	parties  []tss.Party
	partyIDs tss.SortedPartyIDs
	ctx      *tss.PeerContext
)

func main() {
	// setup
	curve = tss.S256()
	count, _ = strconv.Atoi(os.Getenv("COUNT"))
	threshold, _ = strconv.Atoi(os.Getenv("THRESHOLD"))
	parties = make([]tss.Party, count)

	// get ids for parties
	partyIDs = tss.GenerateTestPartyIDs(count)
	ctx = tss.NewPeerContext(partyIDs)

	// use gg18 dkg
	keygenSaves := execKeygen()

	// use gg18 tss
	message := big.NewInt(int64(rand.Int()))
	signature := execSigning(message, keygenSaves)

	fmt.Printf("\n\n")
	fmt.Printf("Message: %s\n", message.String())
	fmt.Printf("Public key (compressed): %s\n", keygenSaves[0].ECDSAPub.X().String())
	fmt.Printf("Signature: %s\n", signature)
}

func execKeygen() []keygen.LocalPartySaveData {
	// communication channel setup
	outCh := make(chan tss.Message, len(partyIDs))
	endCh := make(chan keygen.LocalPartySaveData, len(partyIDs))
	errCh := make(chan *tss.Error, len(partyIDs))

	// initialize parties
	for i := 0; i < len(partyIDs); i++ {
		preParams, _ := keygen.GeneratePreParams(1 * time.Minute)
		params := tss.NewParameters(curve, ctx, partyIDs[i], len(partyIDs), threshold)
		party := keygen.NewLocalParty(params, outCh, endCh, *preParams).(*keygen.LocalParty)
		partyIndex := party.PartyID().Index
		parties[partyIndex] = party
		go func(party *keygen.LocalParty) {
			if err := party.Start(); err != nil {
				errCh <- err
			}
		}(party)
	}

	// execute protocol
	var output []keygen.LocalPartySaveData
	for { // continuous loop
		select {
		case err := <-errCh:
			log.Fatalf("Error occurred during peer communication: %+v\n", err)
		case msg := <-outCh:
			log.Printf("Out channel received message: %+v\n", msg.String())
			recipients := msg.GetTo()
			if recipients == nil { // broadcast
				for _, party := range parties {
					go test.SharedPartyUpdater(party, msg, errCh)
				}
			} else { // p2p
				go test.SharedPartyUpdater(parties[recipients[0].Index], msg, errCh)
			}
		case save := <-endCh:
			index, err := save.OriginalIndex()
			if err != nil {
				log.Fatalf("Error occurred in save.OriginalIndex: %+v\n", err)
			}

			data, _ := json.Marshal(&save)
			path, _ := filepath.Abs("./data/" + strconv.Itoa(index) + "_save.json")
			if _, err = os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
				_ = os.MkdirAll(filepath.Dir(path), fs.ModePerm)
			}
			if err = os.WriteFile(path, data, 0600); err != nil {
				log.Fatalf("Error occurred in os.WriteFile: %+v\n", err)
			}
			log.Printf("Keygen data saved (%s).", path)

			if output = append(output, save); len(output) == len(parties) {
				return output
			}
		}
	}
}

func execSigning(message *big.Int, keygenSaves []keygen.LocalPartySaveData) []byte {
	// communication channel setup
	outCh := make(chan tss.Message, len(partyIDs))
	endCh := make(chan common.SignatureData, len(partyIDs))
	errCh := make(chan *tss.Error, len(partyIDs))

	// initialize parties
	for i := 0; i < len(partyIDs); i++ {
		params := tss.NewParameters(curve, ctx, partyIDs[i], len(partyIDs), threshold)
		party := signing.NewLocalParty(message, params, keygenSaves[i], outCh, endCh).(*signing.LocalParty)
		partyIndex := party.PartyID().Index
		parties[partyIndex] = party
		go func(party *signing.LocalParty) {
			if err := party.Start(); err != nil {
				errCh <- err
			}
		}(party)
	}

	// execute protocol
	for { // continuous loop
		select {
		case err := <-errCh:
			log.Fatalf("Error occurred during peer communication: %+v\n", err)
		case msg := <-outCh:
			log.Printf("Out channel received message: %+v\n", msg.String())
			recipients := msg.GetTo()
			if recipients == nil { // broadcast
				for _, party := range parties {
					go test.SharedPartyUpdater(party, msg, errCh)
				}
			} else { // p2p
				go test.SharedPartyUpdater(parties[recipients[0].Index], msg, errCh)
			}
		case <-endCh:
			log.Printf("Signature computed.")
			return (<-endCh).Signature
		}
	}
}
