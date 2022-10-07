package keygen

import (
	"context"

	"codis/pkg/log"
	"codis/pkg/utils"
	"codis/proto/pb"

	"github.com/bnb-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-msgio/protoio"
)

const (
	ID          = "/keygen/0.0.1"
	ServiceName = "codis.keygen"
	MaxDataSize = 32 * 1024
)

type KeygenService struct {
	Host host.Host

	logger *log.Logger
}

func NewKeygenService(h host.Host) *KeygenService {
	logger := log.NewLogger()

	ks := &KeygenService{
		Host:   h,
		logger: logger,
	}
	h.SetStreamHandler(ID, ks.keygenHandler)
	logger.Debug("Keygen protocol stream handler set.")

	return ks
}

func (ks *KeygenService) Keygen(ctx context.Context, args *pb.KeygenArgs, reply *pb.KeygenReply) error {
	infos, err := utils.StringsToInfos(args.Ids)
	if err != nil {
		return err
	}

	for _, info := range infos {
		if info.ID.String() == ks.Host.ID().String() {
			continue
		}

		s, err := ks.Host.NewStream(ctx, info.ID, ID)
		if err != nil {
			return err
		}
		ks.logger.Debug("New stream created with peer %s", info.ID)

		writer := protoio.NewDelimitedWriter(s)
		if err := writer.WriteMsg(args); err != nil {
			return err
		}
	}
	// Don't think we need to run the keygen logic on the host that received RPC call. Instead, the host will just respon
	//if err := ks.keygen(ks.host.ID(), args, reply); err != nil {
	//	return err
	//}
	return nil
}

func (ks *KeygenService) keygenHandler(s network.Stream) {
	reader := protoio.NewDelimitedReader(s, MaxDataSize)

	var args pb.KeygenArgs
	if err := reader.ReadMsg(&args); err != nil {
		ks.logger.Error(err)
		return
	}

	var reply pb.KeygenReply
	if err := ks.keygen(&args, &reply); err != nil {
		ks.logger.Error(err)
		return
	}
}

type inboundMessage struct {
	Data        []byte
	From        tss.PartyID
	IsBroadcast bool
}

func (ks *KeygenService) keygen(args *pb.KeygenArgs, reply *pb.KeygenReply) error {
	return nil
	// 1. Create tss-lib partyIDs from each p2p ID that was passed as a parameter
	// 2. Make sure there is a stream open between this party and the rest of the participating nodes
	// 3. Attach a protobuf reader and writer to each stream and keep track of them in a map
	// 4. Keep threads open for accepting new messages into a channel, so they can be handled

	//p2pCtx := context.Background()
	//
	//var thisPartyID *tss.PartyID
	//unsortedPartyIDs := make(tss.UnSortedPartyIDs, 0, 1)
	//
	//partyIDs := make(map[string]*tss.PartyID)
	//
	//rws := make(map[tss.PartyID]bufio.ReadWriter)
	//
	//inbCh := make(chan inboundMessage, 1)
	//
	//addrs := utils.StringsToAddrs(args.Ids)
	//infos := utils.AddrsToInfos(addrs)
	//for _, info := range infos {
	//	// 1.
	//	key := new(big.Int)
	//	key.SetBytes([]byte(info.ID))
	//	partyID := tss.NewPartyID(info.ID.String(), "", key)
	//	unsortedPartyIDs = append(unsortedPartyIDs, partyID)
	//
	//	if partyID.String() == ks.host.ID().String() {
	//		thisPartyID = partyID
	//		continue // don't need to set up stream for self
	//	}
	//
	//	// 2.
	//	s, err := ks.host.NewStream(p2pCtx, info.ID)
	//	if err != nil {
	//		return err
	//	}
	//
	//	partyIDs[info.ID.String()] = partyID
	//
	//	// 3.
	//	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
	//	rws[*partyID] = *rw
	//
	//	// 4.
	//	go func() {
	//		for {
	//			var data []byte
	//			if _, err := rw.Read(data); err != nil && err != io.EOF {
	//				continue
	//			} else {
	//				remotePartyID := partyIDs[s.Conn().RemotePeer().String()]
	//				inboundMessage := inboundMessage{
	//					Data: data,
	//					From: *remotePartyID,
	//				}
	//				inbCh <- inboundMessage
	//			}
	//		}
	//	}()
	//}
	//
	//sortedPartyIDs := tss.SortPartyIDs(unsortedPartyIDs)
	//
	//// 3. Initialize the tss-lib local party in preparation for running protocol
	//tssCtx := tss.NewPeerContext(sortedPartyIDs)
	//curve := tss.S256()
	//
	//outCh := make(chan tss.Message, 1)
	//endCh := make(chan libkeygen.LocalPartySaveData, 1)
	//errCh := make(chan *tss.Error, 1)
	//
	//preParams, _ := libkeygen.GeneratePreParams(1 * time.Minute)
	//params := tss.NewParameters(curve, tssCtx, thisPartyID, len(sortedPartyIDs), int(args.Threshold))
	//party := libkeygen.NewLocalParty(params, outCh, endCh, *preParams).(*libkeygen.LocalParty)
	//
	//// 4. Start the protocol
	//go func(party *libkeygen.LocalParty) {
	//	if err := party.Start(); err != nil {
	//		errCh <- err
	//	}
	//}(party)
	//
	//// 5. Handle messages
	////var output libkeygen.LocalPartySaveData
	//for {
	//	select {
	//	// outbound messages
	//	case msg := <-outCh:
	//		msgBytes, msgRouting, err := msg.WireBytes()
	//		if err != nil {
	//			return err
	//		}
	//		if msgRouting.IsBroadcast { // broadcast
	//			for _, id := range sortedPartyIDs {
	//				if _, err := rws[*id].Write(msgBytes); err != nil {
	//					return err
	//				}
	//			}
	//		} else { // p2p
	//			recipients := msg.GetTo()
	//			for _, recipient := range recipients {
	//				if _, err := rws[*recipient].Write(msgBytes); err != nil {
	//					return err
	//				}
	//				if err := rws[*recipient].Flush(); err != nil {
	//					return err
	//				}
	//			}
	//		}
	//	// inbound messages
	//	case msg := <-inbCh:
	//		_, err := party.UpdateFromBytes(msg.Data, &msg.From, msg.IsBroadcast)
	//		if err != nil {
	//			return err
	//		}
	//	case <-endCh:
	//		log.Printf("OMG MF YOU DID IT!\n\n%d\n", (<-endCh).P)
	//	// error messages
	//	case err := <-errCh:
	//		return err
	//	}
	//}
}
