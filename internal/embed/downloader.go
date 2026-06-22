package embed

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	ModelName = "all-MiniLM-L6-v2"

	ModelFileName = "model.onnx"
	VocabFileName = "tokenizer.json"

	maxSeqLen = 512
)

func DefaultCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".cache", "grokdocs"), nil
}

var (
	DefaultModelURL = fmt.Sprintf("https://huggingface.co/sentence-transformers/%s/resolve/main/onnx/model.onnx", ModelName)
	DefaultVocabURL = fmt.Sprintf("https://huggingface.co/sentence-transformers/%s/resolve/main/tokenizer.json", ModelName)
)

type ModelFiles struct {
	ModelPath string
	VocabPath string
	ModelDir  string
}

func GetOrDownloadModels(cacheDir string) (*ModelFiles, error) {
	modelDir := filepath.Join(cacheDir, "models", ModelName)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return nil, fmt.Errorf("create model cache dir: %w", err)
	}

	modelPath := filepath.Join(modelDir, ModelFileName)
	vocabPath := filepath.Join(modelDir, VocabFileName)

	missing := false
	for _, p := range []string{modelPath, vocabPath} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			missing = true
			break
		}
	}

	if !missing {
		return &ModelFiles{
			ModelPath: modelPath,
			VocabPath: vocabPath,
			ModelDir:  modelDir,
		}, nil
	}

	if err := downloadFile(DefaultModelURL, modelPath); err != nil {
		return nil, fmt.Errorf("download model: %w", err)
	}

	if err := downloadFile(DefaultVocabURL, vocabPath); err != nil {
		return nil, fmt.Errorf("download tokenizer: %w", err)
	}

	return &ModelFiles{
		ModelPath: modelPath,
		VocabPath: vocabPath,
		ModelDir:  modelDir,
	}, nil
}

func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d downloading %s", resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", destPath, err)
	}
	return nil
}
