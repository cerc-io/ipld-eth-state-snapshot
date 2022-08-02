package fixture

import (
	"os"
	"path/filepath"
)

// TODO: embed some mainnet data
// import "embed"
//_go:embed mainnet_data.tar.gz

// GetChainDataPath returns the absolute paths to chain data in 'fixture/' given the chain (chain | chain2)
func GetChainDataPath(chain string) (string, string) {
	path := filepath.Join("..", "..", "fixture", chain)

	chaindataPath, err := filepath.Abs(path)
	if err != nil {
		panic("cannot resolve path " + path)
	}
	ancientdataPath := filepath.Join(chaindataPath, "ancient")

	if _, err := os.Stat(chaindataPath); err != nil {
		panic("must populate chaindata at " + chaindataPath)
	}

	return chaindataPath, ancientdataPath
}
