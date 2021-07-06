package dagstore

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
)

type LotusMountAPI interface {
	FetchUnsealedPiece(ctx context.Context, pieceCid cid.Cid) (io.ReadCloser, error)
	GetUnpaddedCARSize(pieceCid cid.Cid) (uint64, error)
}

type lotusMountApiImpl struct {
	pieceStore piecestore.PieceStore
	rm         retrievalmarket.RetrievalProviderNode
}

var _ LotusMountAPI = (*lotusMountApiImpl)(nil)

func NewLotusMountAPI(store piecestore.PieceStore, rm retrievalmarket.RetrievalProviderNode) *lotusMountApiImpl {
	return &lotusMountApiImpl{
		pieceStore: store,
		rm:         rm,
	}
}

func (m *lotusMountApiImpl) FetchUnsealedPiece(ctx context.Context, pieceCid cid.Cid) (io.ReadCloser, error) {
	pieceInfo, err := m.pieceStore.GetPieceInfo(pieceCid)
	if err != nil {
		return nil, xerrors.Errorf("failed to fetch pieceInfo: %w", err)
	}

	if len(pieceInfo.Deals) <= 0 {
		return nil, xerrors.New("no storage deals for Piece")
	}

	// prefer an unsealed sector containing the piece if one exists
	for _, deal := range pieceInfo.Deals {
		isUnsealed, err := m.rm.IsUnsealed(ctx, deal.SectorID, deal.Offset.Unpadded(), deal.Length.Unpadded())
		if err != nil {
			continue
		}
		if isUnsealed {
			// UnsealSector will NOT unseal a sector if we already have an unsealed copy lying around.
			reader, err := m.rm.UnsealSector(ctx, deal.SectorID, deal.Offset.Unpadded(), deal.Length.Unpadded())
			if err == nil {
				return reader, nil
			}
		}
	}

	lastErr := xerrors.New("no sectors found to unseal from")
	// if there is no unsealed sector containing the piece, just read the piece from the first sector we are able to unseal.
	for _, deal := range pieceInfo.Deals {
		reader, err := m.rm.UnsealSector(ctx, deal.SectorID, deal.Offset.Unpadded(), deal.Length.Unpadded())
		if err == nil {
			return reader, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (m *lotusMountApiImpl) GetUnpaddedCARSize(pieceCid cid.Cid) (uint64, error) {
	pieceInfo, err := m.pieceStore.GetPieceInfo(pieceCid)
	if err != nil {
		return 0, xerrors.Errorf("failed to fetch pieceInfo, err=%w", err)
	}

	if len(pieceInfo.Deals) <= 0 {
		return 0, xerrors.New("no storage deals for piece")
	}

	len := pieceInfo.Deals[0].Length

	return uint64(len), nil
}
