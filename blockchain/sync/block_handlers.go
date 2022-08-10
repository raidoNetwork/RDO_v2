package sync

import (
	"context"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
	"github.com/raidoNetwork/RDO_v2/p2p"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"io"
	"time"
)

const (
	maxRequestBlocksCount = 2000
	blockRangeLimit       = 500
)

func (s *Service) blockRangeHandler(ctx context.Context, msg interface{}, stream network.Stream) error {
	ctx, cancel := context.WithTimeout(ctx, respTimeout)
	defer cancel()

	setStreamDeadlines(stream)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	req, ok := msg.(*prototype.BlockRequest)
	if !ok {
		return errors.New("Message type is not block request")
	}

	peer := stream.Conn().RemotePeer()
	if err := s.validateBlockRangeHandler(req); err != nil {
		s.cfg.P2P.PeerStore().BadResponse(peer)
		writeCodeToStream(stream, codeValidationError)
		return errors.Wrap(err, "Validation error for block request")
	}

	step := req.Step
	startSlot := req.StartSlot
	endReqSlot := startSlot + req.Count * step
	endSlot := startSlot + blocksPerRequest

	log.Debugf("Write blocks from %d to %d", startSlot, endReqSlot)

	for startSlot <= endReqSlot {
		if endSlot > endReqSlot {
			endSlot = endReqSlot
		}

		err := s.writeBlockRangeToStream(ctx, startSlot, endSlot, stream)
		if err != nil {
			writeCodeToStream(stream, codeInternalError)
			return err
		}

		log.Debugf("Pushed blocks from %d to %d", startSlot, endSlot)

		startSlot = endSlot + 1
		endSlot += blocksPerRequest

		<-ticker.C
	}

	closeStream(stream)

	return nil
}

func (s *Service) validateBlockRangeHandler(blockReq *prototype.BlockRequest) error {
	if blockReq.Count > maxRequestBlocksCount {
		return errors.New("Invalid block count")
	}

	if blockReq.Step > blockRangeLimit || blockReq.Step == 0 {
		return errors.New("Invalid block step")
	}

	end := slot.Ticker().Slot()
	if blockReq.StartSlot > end {
		return errors.New("Invalid block slot")
	}

	return nil
}

func (s *Service) writeBlockRangeToStream(ctx context.Context, startSlot, endSlot uint64, stream network.Stream) error {
	blocks, err := s.cfg.Blockchain.GetBlocksRange(ctx, startSlot, endSlot)
	if err != nil {
		return err
	}

	for _, b := range blocks {
		if err := s.writeBlockToStream(b, stream); err != nil {
			return errors.Wrap(err, "Error writing block")
		}
	}

	return nil
}

func (s *Service) writeBlockToStream(block *prototype.Block, stream network.Stream) error {
	SetWriteDeadline(stream)
	if _, err := stream.Write([]byte{codeSuccess}); err != nil {
		return err
	}

	_, err := s.cfg.P2P.EncodeStream(stream, block)
	return err
}

func (s *Service) sendBlockRangeRequest(ctx context.Context, req *prototype.BlockRequest, pid peer.ID) ([]*prototype.Block, error) {
	stream, err := s.cfg.P2P.CreateStream(ctx, req, p2p.BlockRangeProtocol, pid)
	if err != nil {
		return nil, errors.Wrap(err, "Create stream error")
	}
	defer closeStream(stream)

	log.Debugf("Open %s stream with %s from %d", p2p.BlockRangeProtocol, pid.String(), req.StartSlot)

	if req.Step == 0 {
		return nil, errors.New("Wrong request step given")
	}

	blocks := make([]*prototype.Block, 0)
	totalCount := req.Count * req.Step + 1

	endSlot := req.StartSlot + totalCount
	var prevNum uint64
	for i := uint64(0); ; i++ {
		block, err := s.receiveBlock(stream)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, errors.Wrap(err, "Error receiving block")
		}

		if i >= totalCount || i >= maxRequestBlocksCount {
			log.Errorf("i: %d total: %d max: %d start: %d count: %d", i, totalCount, maxRequestBlocksCount, req.StartSlot, req.Count)
			return nil, errors.New("Invalid response data")
		}

		if block.Num < req.StartSlot || block.Num > endSlot {
			return nil, errors.New("Wrong block slot")
		}

		if i == 0 {
			prevNum = req.StartSlot
		}

		if (prevNum >= block.Num || (block.Num - prevNum) % req.Step != 0) && i != 0  {
			return nil, errors.New("Not ordered response")
		}

		prevNum = block.Num
		blocks = append(blocks, block)
	}

	log.Debugf("Receive blocks from %d to %d", req.StartSlot, endSlot)

	return blocks, nil
}

func (s *Service) receiveBlock(stream network.Stream) (*prototype.Block, error) {
	SetReadDeadline(stream, respTimeout)

	code, errMsg, err := ReadStatusCode(stream)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading status code")
	}

	if code != 0 {
		return nil, errors.New(errMsg)
	}

	block := &prototype.Block{}
	err = s.cfg.P2P.DecodeStream(stream, block)
	if err != nil  {
		return nil, err
	}

	return block, nil
}