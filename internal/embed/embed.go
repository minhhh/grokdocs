//go:build onnx

package embed

import (
	"fmt"
	"math"

	ort "github.com/yalue/onnxruntime_go"
)

type EmbedderOption func(*Embedder)

func WithTokenizer(t Tokenizer) EmbedderOption {
	return func(e *Embedder) {
		e.tokenizer = t
	}
}

type Embedder struct {
	session    *ort.DynamicAdvancedSession
	inputName  string
	outputName string
	tokenizer  Tokenizer
	dim        int
}

func NewEmbedder(modelPath, vocabPath string, opts ...EmbedderOption) (*Embedder, error) {
	tokenizer, err := NewTokenizer(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("new tokenizer: %w", err)
	}

	ort.SetSharedLibraryPath("/usr/local/lib/libonnxruntime.dylib")
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("init onnx runtime: %w", err)
	}

	// Get model metadata to determine input/output names
	inputInfo, outputInfo, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		ort.DestroyEnvironment()
		return nil, fmt.Errorf("get model IO info: %w", err)
	}

	if len(inputInfo) == 0 || len(outputInfo) == 0 {
		ort.DestroyEnvironment()
		return nil, fmt.Errorf("model has no inputs or outputs")
	}

	inputName := inputInfo[0].Name
	outputName := outputInfo[0].Name

	// Determine embedding dimension from output shape
	var dim int
	if len(outputInfo[0].Dimensions) > 0 {
		dim = int(outputInfo[0].Dimensions[len(outputInfo[0].Dimensions)-1])
	}
	if dim == 0 {
		dim = embeddingDim
	}

	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{inputName, inputInfo[1].Name, inputInfo[2].Name},
		[]string{outputName},
		nil,
	)
	if err != nil {
		ort.DestroyEnvironment()
		return nil, fmt.Errorf("new session: %w", err)
	}

	e := &Embedder{
		session:    session,
		inputName:  inputName,
		outputName: outputName,
		tokenizer:  tokenizer,
		dim:        dim,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
}

func (e *Embedder) Embed(text string) ([]float32, error) {
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

	// Mean pooling + L2 normalization
	rawData := outputTensor.GetData()
	batchSize := int(outputTensor.GetShape()[0])
	seqLen := int(outputTensor.GetShape()[1])
	hiddenDim := int(outputTensor.GetShape()[2])

	embedding := meanPool(rawData, batchSize, seqLen, hiddenDim, attentionMask)
	l2Normalize(embedding)

	return embedding, nil
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

func (e *Embedder) Close() error {
	var err error
	if e.session != nil {
		err = e.session.Destroy()
	}
	ort.DestroyEnvironment()
	return err
}

func (e *Embedder) Dim() int {
	return e.dim
}
