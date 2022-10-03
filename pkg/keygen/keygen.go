package keygen

import (
	"codis/proto/pb"
	"context"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-msgio/protoio"
	"log"
)

const (
	ID          = "/keygen/0.0.1"
	ServiceName = "codis.keygen"
	MaxDataSize = 32 * 1024
)

type KeygenService struct {
	Host host.Host
}

func NewKeygenService(h host.Host) *KeygenService {
	ks := &KeygenService{h}
	h.SetStreamHandler(ID, keygenHandler)
	return ks
}

func (ks *KeygenService) Keygen(ctx context.Context, args *pb.KeygenArgs, reply *pb.KeygenReply) error {
	//for _, id := range args.Ids {
	//	if id == ks.Host.ID().String() {
	//		log.Printf("Attempted self-dial. Continuing...\n")
	//		continue
	//	}
	//	s, err := ks.Host.NewStream(ctx, peer.ID(id), ID)
	//	if err != nil {
	//		return err
	//	}
	//	writer := protoio.NewDelimitedWriter(s)
	//	if err := writer.WriteMsg(args); err != nil {
	//		return err
	//	}
	//}
	if err := keygen(args, reply); err != nil {
		return err
	}
	return nil
}

func keygenHandler(s network.Stream) {
	reader := protoio.NewDelimitedReader(s, MaxDataSize)

	var args pb.KeygenArgs
	var reply pb.KeygenReply
	if err := reader.ReadMsg(&args); err != nil {
		log.Printf("Failed to read from the stream: %+v\n", err)
		return
	}
	if err := keygen(&args, &reply); err != nil {
		log.Printf("Error occurred in the keygen protocol: %+v\n", err)
	}
}

func keygen(args *pb.KeygenArgs, reply *pb.KeygenReply) error {
	log.Printf("keygen hit!\n")
	log.Printf("args are as such: %s\n", args.String())
	reply = &pb.KeygenReply{Message: "Hey joe!!\n"}
	return nil
}
