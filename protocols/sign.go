package protocols

import (
	"errors"
	"github.com/milquellc/codis/log"
	"github.com/milquellc/codis/proto/pb"
	"github.com/milquellc/codis/utils"

	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/bnb-chain/tss-lib/common"
	eckg "github.com/bnb-chain/tss-lib/ecdsa/keygen"
	ecs "github.com/bnb-chain/tss-lib/ecdsa/signing"
	edkg "github.com/bnb-chain/tss-lib/eddsa/keygen"
	eds "github.com/bnb-chain/tss-lib/eddsa/signing"
	"github.com/bnb-chain/tss-lib/tss"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	SignPId              = "/sign/0.0.1"
	signStepDirectPId    = "/sign/step/direct/0.0.1"
	signStepBroadcastPId = "/sign/step/broadcast/0.0.1"
	SignSubscription     = "SignSubscription"
)

type SignService struct {
	host         host.Host
	localParty   tss.Party
	localPartyCh chan tss.Party
	outCh        chan tss.Message
	endCh        chan common.SignatureData
	errCh        chan *tss.Error
	partyIdMap   map[peer.ID]*tss.PartyID
	logger       *log.Logger
	mu           *sync.Mutex
	once         *sync.Once
}

func NewSignService(h host.Host) *SignService {
	logger := log.NewLogger()
	ss := &SignService{
		host:         h,
		localParty:   nil,
		localPartyCh: make(chan tss.Party, 1),
		outCh:        make(chan tss.Message, 1),
		endCh:        make(chan common.SignatureData, 1),
		errCh:        make(chan *tss.Error, 1),
		partyIdMap:   make(map[peer.ID]*tss.PartyID),
		logger:       logger,
		mu:           &sync.Mutex{},
		once:         &sync.Once{},
	}

	h.SetStreamHandler(signStepDirectPId, ss.signStepHandlerDirect)
	logger.Debug("sign step (direct) sub-protocol stream handler set")

	h.SetStreamHandler(signStepBroadcastPId, ss.signStepHandlerBroadcast)
	logger.Debug("sign step (broadcast) sub-protocol stream handler set")

	h.SetStreamHandler(SignPId, ss.signHandler)
	logger.Debug("sign protocol stream handler set")

	return ss
}

func (ss *SignService) Sign(ctx context.Context, args *pb.SignArgs, _ *pb.SignReply) error {
	peerIds, err := utils.PeerIdStringsToPeerIds(args.Party)
	if err != nil {
		return err
	}

	// makes a new stream with each peer using the main protocol ID, then writes the args to the stream.
	for _, peerId := range peerIds {
		if peerId.String() == ss.host.ID().String() {
			continue // don't self-dial
		}
		err = func() error {
			s, err := ss.host.NewStream(ctx, peerId, SignPId)
			if err != nil {
				return err
			} else {
				ss.logger.Debug("new stream created with peer %s", peerId.String())
			}
			defer s.Close()

			msg, err := proto.Marshal(args)
			if err != nil {
				return err
			}

			writer := bufio.NewWriter(s)
			if _, err = writer.Write(msg); err != nil {
				_ = s.Reset()
				return err
			}
			if err := writer.Flush(); err != nil {
				_ = s.Reset()
				return err
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}

	// the initiating peer has no peer to call its sign handler, so it calls its own
	go ss.sign(args) // TODO: take and fill the reply

	return nil
}

func SignSubscriptionHandler(ctx context.Context, sub *pubsub.Subscription, service interface{}) {
	logger := log.NewLogger()

	if service == nil {
		logger.Error(errors.New("sign service required but not given"))
		return
	}

	ss := service.(*SignService)
	for {
		m, err := sub.Next(ctx)
		if err != nil {
			logger.Error(err)
			continue
		}

		var args pb.SignArgs
		if err = proto.Unmarshal(m.Message.Data, &args); err != nil {
			logger.Error(err)
			continue
		}

		var reply *pb.SignReply
		if err = ss.Sign(ctx, &args, reply); err != nil {
			logger.Error(err)
			continue
		}
	}
}

func (ss *SignService) signHandler(s network.Stream) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	var args pb.SignArgs
	data, err := io.ReadAll(s)
	if err != nil {
		ss.logger.Error(err)
		return
	}
	err = proto.Unmarshal(data, &args)
	if err != nil {
		ss.logger.Error(err)
		return
	}

	go ss.sign(&args)
}

func (ss *SignService) signStepHandlerDirect(s network.Stream) {
	ss.signStepHandlerCommon(s, false)
}

func (ss *SignService) signStepHandlerBroadcast(s network.Stream) {
	ss.signStepHandlerCommon(s, true)
}

func (ss *SignService) signStepHandlerCommon(s network.Stream, broadcast bool) {
	waitForLocalParty := func() {
		ss.logger.Debug("waiting for local party to be initialized")
		<-ss.localPartyCh
	}
	ss.once.Do(waitForLocalParty)

	data, err := io.ReadAll(s)
	if err != nil {
		ss.logger.Error(err)
		return
	}

	ok, err := ss.localParty.UpdateFromBytes(data, ss.partyIdMap[s.Conn().RemotePeer()], broadcast)
	if !ok {
		ss.errCh <- ss.localParty.WrapError(err)
	}
}

func (ss *SignService) sign(args *pb.SignArgs) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.reset()

	ss.logger.Info("sign started!")
	ss.logger.Info("alg: %s, party: %s", args.Alg, args.Party)

	peerIds, err := utils.PeerIdStringsToPeerIds(args.Party)
	if err != nil {
		ss.logger.Error(err)
		return
	}

	var thisPartyId *tss.PartyID
	var unsortedPartyIds tss.UnSortedPartyIDs
	for _, peerId := range peerIds {
		key := new(big.Int)
		key.SetBytes([]byte(peerId))
		partyId := tss.NewPartyID(peerId.String(), "", key)
		if partyId.Id == ss.host.ID().String() {
			thisPartyId = partyId // needed later constructing local party
		}

		ss.partyIdMap[peerId] = partyId // pairing peerIds to partyIds for routing
		unsortedPartyIds = append(unsortedPartyIds, partyId)
	}
	sortedPartyIds := tss.SortPartyIDs(unsortedPartyIds)

	peerCtx := tss.NewPeerContext(sortedPartyIds)

	messageBytes, err := hex.DecodeString(args.Message)
	if err != nil {
		ss.logger.Error(err)
		return
	}
	message := new(big.Int)
	message.SetBytes(messageBytes)

	if args.Alg == "eddsa" {
		keysave, err := loadEDDSAKeygenSaves(thisPartyId.Index)
		if err != nil {
			ss.logger.Error(err)
			return
		}
		curve := tss.Edwards()
		params := tss.NewParameters(curve, peerCtx, thisPartyId, len(sortedPartyIds), int(args.Threshold))
		ss.localParty = eds.NewLocalParty(message, params, *keysave, ss.outCh, ss.endCh).(*eds.LocalParty)
	} else {
		keysave, err := loadECDSAKeygenSaves(thisPartyId.Index)
		if err != nil {
			ss.logger.Error(err)
			return
		}
		curve := tss.S256()
		params := tss.NewParameters(curve, peerCtx, thisPartyId, len(sortedPartyIds), int(args.Threshold))
		ss.localParty = ecs.NewLocalParty(message, params, *keysave, ss.outCh, ss.endCh).(*ecs.LocalParty)
	}
	ss.localPartyCh <- ss.localParty // local party initialized and safe to use (for step handlers)

	go func(localParty tss.Party, errCh chan *tss.Error) {
		if err := localParty.Start(); err != nil {
			errCh <- err
		}
	}(ss.localParty, ss.errCh)

	for {
		select {
		case err := <-ss.errCh:
			ss.logger.Error(err)
			return
		case msg := <-ss.outCh:
			ss.logger.Debug("out-channel received message: %s", msg.String())
			msgBytes, msgRouting, err := msg.WireBytes()
			if err != nil {
				ss.logger.Error(err)
				return
			} else if msgRouting.IsBroadcast { // use step broadcast sub-protocol
				for peerID := range ss.partyIdMap {
					if peerID == ss.host.ID() {
						continue // don't send message back to self
					}
					if err = stepSign(ss.host, peerID, msgBytes, true); err != nil {
						ss.logger.Error(err)
						return
					}
				}
			} else { // use step direct sub-protocol
				recipients := msg.GetTo()
				for _, recipient := range recipients {
					for peerID, partyID := range ss.partyIdMap {
						if recipient == partyID {
							if err := stepSign(ss.host, peerID, msgBytes, false); err != nil {
								ss.logger.Error(err)
								return
							}
						}
					}
				}
			}
		case save := <-ss.endCh:
			data, err := protojson.Marshal(&save)
			if err != nil {
				ss.logger.Error(err)
				return
			}

			saveFile, _ := filepath.Abs("saves/" + args.Alg + "/signature_" + strconv.Itoa(thisPartyId.Index) + ".json")
			if _, err = os.Stat(filepath.Dir(saveFile)); os.IsNotExist(err) {
				_ = os.MkdirAll(filepath.Dir(saveFile), fs.ModePerm)
			}
			if err = os.WriteFile(saveFile, data, 0600); err != nil {
				ss.logger.Error(err)
				return
			}
			ss.logger.Info("signature file saved")
			return
		}
	}
}

func (ss *SignService) reset() {
	ss.localParty = nil
	ss.localPartyCh = make(chan tss.Party, 1)
	ss.partyIdMap = make(map[peer.ID]*tss.PartyID)
	ss.outCh = make(chan tss.Message, 1)
	ss.endCh = make(chan common.SignatureData, 1)
	ss.errCh = make(chan *tss.Error, 1)
	ss.mu = &sync.Mutex{}
	ss.once = &sync.Once{}
}

func stepSign(host host.Host, peer peer.ID, msg []byte, broadcast bool) error {
	var stream network.Stream
	var err error
	if broadcast {
		stream, err = host.NewStream(context.Background(), peer, signStepBroadcastPId)
	} else {
		stream, err = host.NewStream(context.Background(), peer, signStepDirectPId)
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

func loadECDSAKeygenSaves(index int) (*eckg.LocalPartySaveData, error) {
	path, _ := filepath.Abs("./saves/ecdsa/keysave_" + strconv.Itoa(index) + ".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var keygenSave eckg.LocalPartySaveData
	if err = json.Unmarshal(data, &keygenSave); err != nil {
		return nil, err
	}

	curve := tss.S256()
	for _, kbxj := range keygenSave.BigXj {
		kbxj.SetCurve(curve)
	}
	keygenSave.ECDSAPub.SetCurve(curve)

	return &keygenSave, nil
}

func loadEDDSAKeygenSaves(index int) (*edkg.LocalPartySaveData, error) {
	path, _ := filepath.Abs("./saves/eddsa/keysave_" + strconv.Itoa(index) + ".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var keygenSave edkg.LocalPartySaveData
	if err = json.Unmarshal(data, &keygenSave); err != nil {
		return nil, err
	}

	curve := tss.Edwards()
	for _, kbxj := range keygenSave.BigXj {
		kbxj.SetCurve(curve)
	}
	keygenSave.EDDSAPub.SetCurve(curve)

	return &keygenSave, nil
}
