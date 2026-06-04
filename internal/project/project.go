package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minhhh/grokdocs/internal/config"
)

const (
	ConfigDirName   = ".grokdocs"
	ConfigFileName  = "config.yaml"
	FTSDBFileName   = "grokdocs.db"
	VectorIndexName = "grokdocs.index"
)

// Project represents the grokdocs project workspace.
type Project struct {
	RootPath   string
	ConfigDir  string // Absolute path to .grokdocs directory
	Config     *config.Config
	ftsDB      *FTSDatabase
	vectorDB   *VectorDatabase
}

// NewProject creates a new Project instance for the given root path.
func NewProject(rootPath string) (*Project, error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for root: %w", err)
	}
	return &Project{
		RootPath:  absRoot,
		ConfigDir: filepath.Join(absRoot, ConfigDirName),
	}, nil
}

// FindProject starts at startDir and walks up parent directories searching for a `.grokdocs` folder.
// If none is found up to the filesystem root, it falls back to startDir.
func FindProject(startDir string) (*Project, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute start path: %w", err)
	}

	current := absStart
	for {
		grokDir := filepath.Join(current, ConfigDirName)
		if info, err := os.Stat(grokDir); err == nil && info.IsDir() {
			return NewProject(current)
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root, fallback to startDir
			return NewProject(absStart)
		}
		current = parent
	}
}

// Init initializes the `.grokdocs` directory and writes a default `config.yaml` if not present.
func (p *Project) Init() error {
	// Create .grokdocs directory
	if err := os.MkdirAll(p.ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(p.ConfigDir, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := config.DefaultConfig()
		if err := cfg.SaveToFile(configPath); err != nil {
			return fmt.Errorf("failed to create default config.yaml: %w", err)
		}
	}

	return nil
}

// Load loads and parses the `config.yaml` from the discovered `.grokdocs` directory.
func (p *Project) Load() error {
	configPath := filepath.Join(p.ConfigDir, ConfigFileName)
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config.yaml: %w", err)
	}
	p.Config = cfg
	return nil
}

// OpenFTS opens the FTS SQLite database located in `.grokdocs/grokdocs.db`.
func (p *Project) OpenFTS() (*FTSDatabase, error) {
	if p.ftsDB != nil {
		return p.ftsDB, nil
	}
	dbPath := filepath.Join(p.ConfigDir, FTSDBFileName)
	db, err := OpenFTSDatabase(dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.InitSchema(); err != nil {
		db.Close()
		return nil, err
	}
	p.ftsDB = db
	return db, nil
}

// OpenVector opens (or initializes) the FAISS vector database located in `.grokdocs/grokdocs.index`.
func (p *Project) OpenVector() (*VectorDatabase, error) {
	if p.vectorDB != nil {
		return p.vectorDB, nil
	}
	indexPath := filepath.Join(p.ConfigDir, VectorIndexName)
	db, err := OpenVectorDatabase(indexPath)
	if err != nil {
		return nil, err
	}
	p.vectorDB = db
	return db, nil
}

// Close closes any open database connections.
func (p *Project) Close() error {
	var ftsErr, vecErr error
	if p.ftsDB != nil {
		ftsErr = p.ftsDB.Close()
		p.ftsDB = nil
	}
	if p.vectorDB != nil {
		vecErr = p.vectorDB.Close()
		p.vectorDB = nil
	}
	if ftsErr != nil {
		return ftsErr
	}
	return vecErr
}
