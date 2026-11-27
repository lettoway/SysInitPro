package assets

import (
	"SysInitPro/internal/models"
	_ "embed"
	"encoding/json"
)

//go:embed openwrt_catalog.json
var openwrtCatalogBytes []byte

func LoadOpenWrtCatalog() (models.OpenWrtCatalog, error) {
	var cat models.OpenWrtCatalog
	err := json.Unmarshal(openwrtCatalogBytes, &cat)
	return cat, err
}
