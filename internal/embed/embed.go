//go:build onnx

package embed

import (
	"fmt"
	"math"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

type Embedder struct {
	session   *ort.DynamicAdvancedSession
	tokenizer Tokenizer
	dim       int
}

var (
	globalEmbedder   *Embedder
	globalEmbedderMu sync.Mutex
)

func GetGlobalEmbedder() (*Embedder, error) {
	globalEmbedderMu.Lock()
	defer globalEmbedderMu.Unlock()

	if globalEmbedder != nil {
		return globalEmbedder, nil
	}

	cacheDir, err := DefaultCacheDir()
	if err != nil {
		return nil, fmt.Errorf("cache dir: %w", err)
	}
	mf, err := GetOrDownloadModels(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("get models: %w", err)
	}

	ort.SetSharedLibraryPath("/usr/local/lib/libonnxruntime.dylib")
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("init onnx runtime: %w", err)
	}

	inputInfo, outputInfo, err := ort.GetInputOutputInfo(mf.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("get model IO info: %w", err)
	}
	if len(inputInfo) == 0 || len(outputInfo) == 0 {
		return nil, fmt.Errorf("model has no inputs or outputs")
	}

	var inputNames []string
	for _, in := range inputInfo {
		inputNames = append(inputNames, in.Name)
	}
	var outputNames []string
	for _, out := range outputInfo {
		outputNames = append(outputNames, out.Name)
	}

	dim := int(outputInfo[0].Dimensions[len(outputInfo[0].Dimensions)-1])

	tokenizer, err := NewTokenizer(mf.VocabPath)
	if err != nil {
		return nil, fmt.Errorf("new tokenizer: %w", err)
	}

	session, err := ort.NewDynamicAdvancedSession(mf.ModelPath, inputNames, outputNames, nil)
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}

	globalEmbedder = &Embedder{
		session:   session,
		tokenizer: tokenizer,
		dim:       dim,
	}
	return globalEmbedder, nil
}

func Embed(text string) ([]float32, error) {
	e, err := GetGlobalEmbedder()
	if err != nil {
		return nil, err
	}

	inputIDs, attentionMask, tokenTypeIDs := e.tokenizer.Encode(text, maxSeqLen)

	inIDs, err := ort.NewTensor(ort.NewShape(1, int64(maxSeqLen)), inputIDs)
	if err != nil {
		return nil, fmt.Errorf("create input_ids tensor: %w", err)
	}
	defer inIDs.Destroy()

	attnMask, err := ort.NewTensor(ort.NewShape(1, int64(maxSeqLen)), attentionMask)
	if err != nil {
		return nil, fmt.Errorf("create attention_mask tensor: %w", err)
	}
	defer attnMask.Destroy()

	tokTypeIDs, err := ort.NewTensor(ort.NewShape(1, int64(maxSeqLen)), tokenTypeIDs)
	if err != nil {
		return nil, fmt.Errorf("create token_type_ids tensor: %w", err)
	}
	defer tokTypeIDs.Destroy()

	outputs := []ort.Value{nil}
	if err := e.session.Run(
		[]ort.Value{inIDs, attnMask, tokTypeIDs},
		outputs,
	); err != nil {
		return nil, fmt.Errorf("run session: %w", err)
	}
	defer outputs[0].Destroy()

	outputTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output type")
	}

	rawData := outputTensor.GetData()
	batchSize := int(outputTensor.GetShape()[0])
	seqLen := int(outputTensor.GetShape()[1])
	hiddenDim := int(outputTensor.GetShape()[2])

	embedding := meanPool(rawData, batchSize, seqLen, hiddenDim, attentionMask)
	l2Normalize(embedding)

	return embedding, nil
}

func Dim() int {
	e, err := GetGlobalEmbedder()
	if err != nil {
		return 0
	}
	return e.dim
}

var closeOnce sync.Once

func Close() {
	closeOnce.Do(func() {
		globalEmbedderMu.Lock()
		if globalEmbedder != nil && globalEmbedder.session != nil {
			globalEmbedder.session.Destroy()
		}
		globalEmbedderMu.Unlock()
		ort.DestroyEnvironment()
	})
}

func meanPool(data []float32, batchSize, seqLen, hiddenDim int, attentionMask []int64) []float32 {
	embedding := make([]float32, hiddenDim)

	for b := 0; b < batchSize; b++ {
		var maskSum float32
		for s := 0; s < seqLen; s++ {
			if attentionMask[s] > 0 {
				maskSum++
			}
		}
		if maskSum == 0 {
			continue
		}

		for s := 0; s < seqLen; s++ {
			if attentionMask[s] == 0 {
				continue
			}
			for h := 0; h < hiddenDim; h++ {
				idx := b*seqLen*hiddenDim + s*hiddenDim + h
				embedding[h] += data[idx] / maskSum
			}
		}
	}

	return embedding
}

func l2Normalize(vec []float32) {
	var sum float32
	for _, v := range vec {
		sum += v * v
	}
	norm := float32(math.Sqrt(float64(sum)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
}
