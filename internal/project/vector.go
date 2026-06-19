package project

import (
	"fmt"
	"os"

	faiss "github.com/DataIntelligenceCrew/go-faiss"
	"github.com/minhhh/grokdocs/internal/util"
)

type VectorDatabase struct {
	IndexPath string
	index     faiss.Index
}

func OpenVectorDatabase(indexPath string, dim int) (*VectorDatabase, error) {
	var index faiss.Index
	if _, err := os.Stat(indexPath); err == nil {
		idx, err := faiss.ReadIndex(indexPath, 0)
		if err != nil {
			return nil, fmt.Errorf("read faiss index %s: %w", indexPath, err)
		}
		index = idx
		util.Logger.Debug().Str("path", indexPath).Int("d", index.D()).Int64("ntotal", index.Ntotal()).Msg("loaded existing FAISS index")
	} else {
		idx, err := faiss.IndexFactory(dim, "IDMap,Flat", faiss.MetricL2)
		if err != nil {
			return nil, fmt.Errorf("create faiss index: %w", err)
		}
		index = idx
		util.Logger.Debug().Str("path", indexPath).Int("d", dim).Msg("created new FAISS index")
	}

	return &VectorDatabase{
		IndexPath: indexPath,
		index:     index,
	}, nil
}

func (v *VectorDatabase) AddVectors(ids []int64, vectors []float32) error {
	if len(ids) == 0 || len(vectors) == 0 {
		return nil
	}
	if err := v.index.AddWithIDs(vectors, ids); err != nil {
		return fmt.Errorf("add vectors to faiss: %w", err)
	}
	return nil
}

func (v *VectorDatabase) RemoveIDs(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	sel, err := faiss.NewIDSelectorBatch(ids)
	if err != nil {
		return fmt.Errorf("new id selector: %w", err)
	}
	defer sel.Delete()
	if _, err := v.index.RemoveIDs(sel); err != nil {
		return fmt.Errorf("remove ids: %w", err)
	}
	return nil
}

func (v *VectorDatabase) Search(query []float32, k int) ([]int64, []float32, error) {
	ntotal := v.index.Ntotal()
	if ntotal == 0 {
		return nil, nil, nil
	}
	if int64(k) > ntotal {
		k = int(ntotal)
	}
	distances, labels, err := v.index.Search(query, int64(k))
	if err != nil {
		return nil, nil, fmt.Errorf("faiss search: %w", err)
	}
	return labels, distances, nil
}

func (v *VectorDatabase) Save() error {
	if err := faiss.WriteIndex(v.index, v.IndexPath); err != nil {
		util.Logger.Error().Err(err).Str("path", v.IndexPath).Msg("failed to save FAISS index")
		return fmt.Errorf("write faiss index: %w", err)
	}
	util.Logger.Debug().Str("path", v.IndexPath).Int64("ntotal", v.index.Ntotal()).Msg("saved FAISS index")
	return nil
}

func (v *VectorDatabase) Close() error {
	if v.index != nil {
		v.index.Delete()
		v.index = nil
	}
	return nil
}

func (v *VectorDatabase) Dim() int {
	if v.index == nil {
		return 0
	}
	return v.index.D()
}

func (v *VectorDatabase) Len() int64 {
	if v.index == nil {
		return 0
	}
	return v.index.Ntotal()
}
