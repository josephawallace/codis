package protocols

import (
	"encoding/hex"
	"github.com/milquellc/codis/database"
	"github.com/milquellc/codis/log"
	"github.com/milquellc/codis/proto/pb"
	"github.com/milquellc/codis/utils"

	"bufio"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	eckg "github.com/bnb-chain/tss-lib/ecdsa/keygen"
	edkg "github.com/bnb-chain/tss-lib/eddsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	KeygenPId              = "/keygen/0.0.1"
	keygenStepDirectPId    = "/keygen/step/direct/0.0.1"
	keygenStepBroadcastPId = "/keygen/step/broadcast/0.0.1"
	KeygenTopic            = "KeygenTopic"
)

// KeygenService defines the state and functions needed to perform distributed key generation.
type KeygenService struct {
	host         host.Host
	localParty   tss.Party
	localPartyCh chan tss.Party
	outCh        chan tss.Message
	ecEndCh      chan eckg.LocalPartySaveData
	edEndCh      chan edkg.LocalPartySaveData
	errCh        chan *tss.Error
	partyIdMap   map[peer.ID]*tss.PartyID
	logger       *log.Logger
	mu           *sync.Mutex
	once         *sync.Once
}

// NewKeygenService constructs the initial state for the keygen service, sets the stream handlers for the protocols
// leveraged by this service, then returns the service object. This function should be used for any peer that intends to
// handle keygen requests. It is currently being used when setting up a peer's RPC server, as all peers with RPC servers
// should expect keygen requests, and those without (i.e. bootstrap nodes, clients) should not.
func NewKeygenService(h host.Host) *KeygenService {
	logger := log.NewLogger()
	ks := &KeygenService{
		host:         h,
		localParty:   nil,
		localPartyCh: make(chan tss.Party, 1),
		outCh:        make(chan tss.Message, 1),
		ecEndCh:      make(chan eckg.LocalPartySaveData, 1),
		edEndCh:      make(chan edkg.LocalPartySaveData, 1),
		errCh:        make(chan *tss.Error, 1),
		partyIdMap:   make(map[peer.ID]*tss.PartyID),
		logger:       logger,
		mu:           &sync.Mutex{},
		once:         &sync.Once{},
	}

	h.SetStreamHandler(keygenStepDirectPId, ks.keygenStepHandlerDirect)
	logger.Debug("keygen step (direct) sub-protocol stream handler set")

	h.SetStreamHandler(keygenStepBroadcastPId, ks.keygenStepHandlerBroadcast)
	logger.Debug("keygen step (broadcast) sub-protocol stream handler set")

	h.SetStreamHandler(KeygenPId, ks.keygenHandler)
	logger.Debug("keygen protocol stream handler set")

	return ks
}

// Keygen is the entrypoint for a host to execute a keygen protocol from an RPC call. A client would fill out the
// argument structure and send it via RPC call to a host. The args should contain info like what the count and
// threshold are for the party and the multiaddrs of each participant. Using these details, we can expect our host to
// execute the protocol and fill out the reply structure to be read back by the client.
func (ks *KeygenService) Keygen(ctx context.Context, args *pb.KeygenArgs, reply *pb.KeygenReply) error {
	go ks.keygen(args, reply) // TODO: take and fill the reply
	return nil
}

// keygenHandler is the entrypoint for a peer to execute a keygen protocol on an incoming stream. This function would
// run as a result of a peer opening a new stream with this peer using the KeygenPId protocol ID. All this function does
// is read and parse the args from the stream; the real workload of the protocol is deferred to the keygen function
// further down.
func (ks *KeygenService) keygenHandler(s network.Stream) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	var (
		args  pb.KeygenArgs
		reply pb.KeygenReply
	)

	data, err := io.ReadAll(s)
	if err != nil {
		ks.logger.Error(err)
		return
	}
	err = proto.Unmarshal(data, &args)
	if err != nil {
		ks.logger.Error(err)
		return
	}

	go ks.keygen(&args, &reply)
}

// keygenStepHandlerDirect reads tss messages sent to this peer directly and updates the local party
func (ks *KeygenService) keygenStepHandlerDirect(s network.Stream) {
	ks.keygenStepHandlerCommon(s, false)
}

// keygenStepHandlerBroadcast reads tss messages sent as a broadcast and updates the local party
func (ks *KeygenService) keygenStepHandlerBroadcast(s network.Stream) {
	ks.keygenStepHandlerCommon(s, true)
}

// keygenStepHandlerCommon does the work for the real stream handlers. The logic is broken out of the stream handlers
// themselves only because the code is so similar.
func (ks *KeygenService) keygenStepHandlerCommon(s network.Stream, broadcast bool) {
	// the keygen function, where most of the work is being done, is run as a goroutine. in it, the local party is
	// constructed and set. since that function is running asynchronously, this function may be invoked by another peer
	// opening a step stream, before the local party has had a chance to be initialized. the ks.localPartyCh is only
	// used to block further execution until the local party has been initialized.
	waitForLocalParty := func() {
		ks.logger.Debug("waiting for local party to be initialized")
		<-ks.localPartyCh
	}
	ks.once.Do(waitForLocalParty)

	data, err := io.ReadAll(s)
	if err != nil {
		ks.logger.Error(err)
		return
	}

	ok, err := ks.localParty.UpdateFromBytes(data, ks.partyIdMap[s.Conn().RemotePeer()], broadcast)
	if !ok {
		ks.errCh <- ks.localParty.WrapError(err)
	}
}

// keygen is where most of the tss-lib related stuff happens (the step handlers are the exception, but are supposed to
// only be invoked on other peers by this function anyway). The local party is set up and started, then there is a
// select statement that gets outbound messages from the local party as they come, then sends the messages via the step
// sub-protocols.
func (ks *KeygenService) keygen(args *pb.KeygenArgs, reply *pb.KeygenReply) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	ks.reset()

	ks.logger.Info("keygen started!")
	ks.logger.Info("count: %d, threshold: %d, alg: %s, party: %s", args.Count, args.Threshold, args.Algorithm, args.Party)

	peerIds, err := utils.PeerIdStringsToPeerIds(args.Party)
	if err != nil {
		ks.logger.Error(err)
		return
	}

	var thisPartyId *tss.PartyID
	var unsortedPartyIds tss.UnSortedPartyIDs
	for _, peerId := range peerIds {
		key := new(big.Int)
		key.SetBytes([]byte(peerId))
		partyId := tss.NewPartyID(peerId.String(), "", key)
		if partyId.Id == ks.host.ID().String() {
			thisPartyId = partyId // needed later constructing local party
		}

		ks.partyIdMap[peerId] = partyId // pairing peerIds to partyIds for routing
		unsortedPartyIds = append(unsortedPartyIds, partyId)
	}
	sortedPartyIds := tss.SortPartyIDs(unsortedPartyIds)

	peerCtx := tss.NewPeerContext(sortedPartyIds)

	if string(args.Algorithm) == "eddsa" {
		curve := tss.Edwards()
		params := tss.NewParameters(curve, peerCtx, thisPartyId, len(sortedPartyIds), int(args.Threshold))
		ks.localParty = edkg.NewLocalParty(params, ks.outCh, ks.edEndCh).(*edkg.LocalParty)
	} else {
		curve := tss.S256()
		preParams, err := eckg.GeneratePreParams(1 * time.Minute)
		if err != nil {
			ks.logger.Error(err)
			return
		}
		params := tss.NewParameters(curve, peerCtx, thisPartyId, len(sortedPartyIds), int(args.Threshold))
		ks.localParty = eckg.NewLocalParty(params, ks.outCh, ks.ecEndCh, *preParams).(*eckg.LocalParty)
	}
	ks.localPartyCh <- ks.localParty // local party initialized and safe to use (for step handlers)

	go func(localParty tss.Party, errCh chan *tss.Error) {
		if err := localParty.Start(); err != nil {
			errCh <- err
		}
	}(ks.localParty, ks.errCh)

	for {
		select {
		case err := <-ks.errCh:
			ks.logger.Error(err)
			return
		case msg := <-ks.outCh:
			ks.logger.Debug("out-channel received message: %s", msg.String())
			msgBytes, msgRouting, err := msg.WireBytes()
			if err != nil {
				ks.logger.Error(err)
				return
			} else if msgRouting.IsBroadcast { // use step broadcast sub-protocol
				for peerID := range ks.partyIdMap {
					if peerID == ks.host.ID() {
						continue // don't send message back to self
					}
					if err = stepKeygen(ks.host, peerID, msgBytes, true); err != nil {
						ks.logger.Error(err)
						return
					}
				}
			} else { // use step direct sub-protocol
				recipients := msg.GetTo()
				for _, recipient := range recipients {
					for peerID, partyID := range ks.partyIdMap {
						if recipient == partyID {
							// 5.
							if err := stepKeygen(ks.host, peerID, msgBytes, false); err != nil {
								ks.logger.Error(err)
								return
							}
						}
					}
				}
			}
		case save := <-ks.ecEndCh:
			data, err := json.Marshal(&save)
			if err != nil {
				ks.logger.Error(err)
				return
			}
			keygenSave := pb.KeygenSaveData{
				Data:     data,
				Metadata: args,
			}

			key := append(save.ECDSAPub.ToECDSAPubKey().X.Bytes(), save.ECDSAPub.ToECDSAPubKey().Y.Bytes()...)
			val, err := proto.Marshal(&keygenSave)
			if err != nil {
				ks.logger.Error(err)
				return
			}
			if err = database.Set(key, val); err != nil {
				ks.logger.Error(err)
				return
			}
			return
		case save := <-ks.edEndCh:
			data, err := json.Marshal(&save)
			if err != nil {
				ks.logger.Error(err)
				return
			}
			keygenSave := pb.KeygenSaveData{
				Data:     data,
				Metadata: args,
			}

			key := append(save.EDDSAPub.ToECDSAPubKey().X.Bytes(), save.EDDSAPub.ToECDSAPubKey().Y.Bytes()...)
			val, err := proto.Marshal(&keygenSave)
			if err != nil {
				ks.logger.Error(err)
				return
			}

			if err = database.Set(key, val); err != nil {
				ks.logger.Error(err)
				return
			}

			ks.logger.Info("saved key %s", hex.EncodeToString(key))
			return
		}
	}
}

// reset clears the fields of the keygen service state that are involved in the keygen protocol execution.
func (ks *KeygenService) reset() {
	ks.localParty = nil
	ks.localPartyCh = make(chan tss.Party, 1)
	ks.partyIdMap = make(map[peer.ID]*tss.PartyID)
	ks.outCh = make(chan tss.Message, 1)
	ks.ecEndCh = make(chan eckg.LocalPartySaveData, 1)
	ks.edEndCh = make(chan edkg.LocalPartySaveData, 1)
	ks.errCh = make(chan *tss.Error, 1)
	ks.mu = &sync.Mutex{}
	ks.once = &sync.Once{}
}

// stepKeygen creates the streams for the step sub-protocols. These essentially comprise the "messaging" component of
// the keygen protocol. When there is an outbound message, streams are opened, the message is written to them, and then
// the peer on the other end reads from the stream and updates their local party's state based on the message.
func stepKeygen(host host.Host, peer peer.ID, msg []byte, broadcast bool) error {
	var stream network.Stream
	var err error
	if broadcast {
		stream, err = host.NewStream(context.Background(), peer, keygenStepBroadcastPId)
	} else {
		stream, err = host.NewStream(context.Background(), peer, keygenStepDirectPId)
	}
	if err != nil {
		return err
	}
	defer stream.Close()

	writer := bufio.NewWriter(stream)
	if _, err = writer.Write(msg); err != nil {
		_ = stream.Reset()
		return err
	}
	if err := writer.Flush(); err != nil {
		_ = stream.Reset()
		return err
	}
	return nil
}
