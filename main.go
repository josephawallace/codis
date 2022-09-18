package main

import (
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/ecdsa/keygen"
	"github.com/binance-chain/tss-lib/test"
	"github.com/binance-chain/tss-lib/tss"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	keygenSaves := execKeygen(3, 2)
	fmt.Printf("\n\n")
	for index, save := range keygenSaves {
		fmt.Printf("Public key output (party %d): %s\n", index, save.ECDSAPub.X().String())
	}
	fmt.Printf("\n")
}

func execKeygen(count int, threshold int) []keygen.LocalPartySaveData {
	// configuration
	curve := tss.S256()

	// get ids for parties
	partyIDs := tss.GenerateTestPartyIDs(count)
	ctx := tss.NewPeerContext(partyIDs)

	// communication channel setup
	outCh := make(chan tss.Message, len(partyIDs))
	endCh := make(chan keygen.LocalPartySaveData, len(partyIDs))
	errCh := make(chan *tss.Error, len(partyIDs))

	// initialize parties
	parties := make(map[string]tss.Party)
	for i := 0; i < len(partyIDs); i++ {
		preParams, _ := keygen.GeneratePreParams(1 * time.Minute)
		params := tss.NewParameters(curve, ctx, partyIDs[i], len(partyIDs), threshold)
		party := keygen.NewLocalParty(params, outCh, endCh, *preParams)
		partyID := party.PartyID().Id
		parties[partyID] = party
		go func(party tss.Party) {
			if err := party.Start(); err != nil {
				errCh <- err
			}
		}(party)
	}

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
				go test.SharedPartyUpdater(parties[recipients[0].Id], msg, errCh)
			}
		case save := <-endCh:
			index, err := save.OriginalIndex()
			if err != nil {
				log.Fatalf("Error occurred in save.OriginalIndex: %+v\n", err)
			}

			data, _ := json.Marshal(&save)
			path, _ := filepath.Abs("./data/" + strconv.Itoa(index) + "_save.json")
			if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
				_ = os.MkdirAll(filepath.Dir(path), fs.ModePerm)
			}
			if err = os.WriteFile(path, data, 0600); err != nil {
				log.Fatalf("Error occurred in os.WriteFile: %+v\n", err)
			}
			log.Printf("Post-protocol data saved.")

			if output = append(output, save); len(output) == len(parties) {
				return output
			}
			break
		}
	}
}
