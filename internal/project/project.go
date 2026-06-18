package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minhhh/grokdocs/internal/config"
	"github.com/minhhh/grokdocs/internal/util"
)

const (
	ConfigDirName   = ".grokdocs"
	ConfigFileName  = "config.yaml"
	FTSDBFileName   = "grokdocs.db"
	VectorIndexName = "grokdocs.index"
)

// Project represents the grokdocs project workspace.
type Project struct {
	RootPath      string
	ConfigDir     string // Absolute path to .grokdocs directory
	Config        *config.Config
	ftsDB         *FTSDatabase
	vectorDB      *VectorDatabase
	collVectorDBs map[string]*VectorDatabase
}

// NewProject creates a new Project instance for the given root path.
func NewProject(rootPath string) (*Project, error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		util.Logger.Error().Err(err).Str("path", rootPath).Msg("failed to get absolute path for root")
		return nil, err
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
		util.Logger.Error().Err(err).Str("path", startDir).Msg("failed to resolve absolute start path")
		return nil, err
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

// Init initializes the `.grokdocs` directory, writes a default
// `config.yaml` if not present, and loads the configuration.
func (p *Project) Init() error {
	if info, err := os.Stat(p.RootPath); err != nil {
		if os.IsNotExist(err) {
			util.Logger.Error().Str("path", p.RootPath).Msg("project root does not exist")
			return err
		}
		util.Logger.Error().Err(err).Str("path", p.RootPath).Msg("failed to stat project root")
		return err
	} else if !info.IsDir() {
		util.Logger.Error().Str("path", p.RootPath).Msg("project root is not a directory")
		return errors.New("project root is not a directory")
	}

	if err := os.MkdirAll(p.ConfigDir, 0755); err != nil {
		util.Logger.Error().Err(err).Str("path", p.ConfigDir).Msg("failed to create config directory")
		return err
	}

	configPath := filepath.Join(p.ConfigDir, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := config.DefaultConfig()
		if err := cfg.SaveToFile(configPath); err != nil {
			util.Logger.Error().Err(err).Str("path", configPath).Msg("failed to create default config.yaml")
			return err
		}
	}

	return p.load()
}

// load loads and parses the `config.yaml` from the `.grokdocs` directory.
func (p *Project) load() error {
	configPath := filepath.Join(p.ConfigDir, ConfigFileName)
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		util.Logger.Error().Err(err).Str("path", configPath).Msg("failed to load config.yaml")
		return err
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

// CollectionIndexName returns the FAISS index filename for a given collection.
func CollectionIndexName(collection string) string {
	return fmt.Sprintf("grokdocs-%s.index", collection)
}

// OpenCollectionVector opens (or initializes) the per-collection FAISS vector database.
func (p *Project) OpenCollectionVector(collection string) (*VectorDatabase, error) {
	if p.collVectorDBs == nil {
		p.collVectorDBs = make(map[string]*VectorDatabase)
	}
	if vdb, ok := p.collVectorDBs[collection]; ok {
		return vdb, nil
	}
	indexPath := filepath.Join(p.ConfigDir, CollectionIndexName(collection))
	vdb, err := OpenVectorDatabase(indexPath)
	if err != nil {
		return nil, err
	}
	p.collVectorDBs[collection] = vdb
	return vdb, nil
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
	for _, vdb := range p.collVectorDBs {
		if err := vdb.Close(); err != nil {
			vecErr = err
		}
	}
	p.collVectorDBs = nil
	if ftsErr != nil {
		return ftsErr
	}
	return vecErr
}
