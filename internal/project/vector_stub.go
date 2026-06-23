//go:build !onnx

package project

import "errors"

type VectorDatabase struct {
	IndexPath string
}

func OpenVectorDatabase(_ string, _ int) (*VectorDatabase, error) {
	return nil, errors.New("vector search requires building with -tags onnx")
}

func (v *VectorDatabase) AddVectors(_ []int64, _ []float32) error {
	return errors.New("vector search requires building with -tags onnx")
}

func (v *VectorDatabase) Reset() error {
	return errors.New("vector search requires building with -tags onnx")
}

func (v *VectorDatabase) RemoveIDs(_ []int64) error {
	return errors.New("vector search requires building with -tags onnx")
}

func (v *VectorDatabase) Search(_ []float32, _ int) ([]int64, []float32, error) {
	return nil, nil, errors.New("vector search requires building with -tags onnx")
}

func (v *VectorDatabase) Save() error {
	return errors.New("vector search requires building with -tags onnx")
}

func (v *VectorDatabase) Close() error {
	return nil
}

func (v *VectorDatabase) Dim() int {
	return 0
}

func (v *VectorDatabase) Len() int64 {
	return 0
}
