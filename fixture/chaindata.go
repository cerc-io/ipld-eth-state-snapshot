package fixture

import (
	"os"
	"path/filepath"
	"runtime"
)

// TODO: embed some mainnet data
// import "embed"
//_go:embed mainnet_data.tar.gz

var (
	ChaindataPath, AncientdataPath string
)

func init() {
	_, path, _, _ := runtime.Caller(0)
	wd := filepath.Dir(path)

	ChaindataPath = filepath.Join(wd, "..", "fixture", "chaindata")
	AncientdataPath = filepath.Join(ChaindataPath, "ancient")

	if _, err := os.Stat(ChaindataPath); err != nil {
		panic("must populate chaindata at " + ChaindataPath)
	}
}
